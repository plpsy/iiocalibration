package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

const (
	cfgFilePath = "/media/sd-mmcblk1p2/calibration.json"
	caliSamples = "1024"
)

type CaliParams struct {
	ChanOff [15]int `json:"ChanOff"`
}

func CalibrationParams(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	caliparams := CaliParams{}

	writeResponse(w, caliparams)
}

// Writes the response as a standard JSON response with StatusOK
func writeResponse(w http.ResponseWriter, m interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(m); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Internal Server Error")
	}
}

// Writes the error response as a Standard API JSON response with a response code
func writeErrorResponse(w http.ResponseWriter, errorCode int, errorMsg string) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(errorCode)
	json.NewEncoder(w).Encode(errorMsg)
}

func WeightLoad(w http.ResponseWriter, r *http.Request, params httprouter.Params) {

	r.ParseMultipartForm(32 << 20)
	// 根据字段名获取表单文件
	formFile, _, err := r.FormFile("weight")
	if err != nil {
		logrus.Errorf("WeightLoad get form file failed: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer formFile.Close()

	// 创建保存文件
	path := "/tmp/weight.bin"

	os.Remove(path)

	destFile, err := os.Create(path)
	if err != nil {
		logrus.Errorf("WeightLoad crate file failed: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer destFile.Close()

	if _, err = formFile.Seek(0, io.SeekStart); err != nil {
		logrus.Errorf("WeightLoad seek failed: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 读取表单文件，写入保存文件
	_, err = io.Copy(destFile, formFile)
	if err != nil {
		logrus.Errorf("WeightLoad copy file failed: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	weightLoadWrapper(w, r, params)
}

func Calibration(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	vars := r.URL.Query()
	channel, ok := vars["channel"]
	if !ok {
		err := calibrationAll()
		if err != nil {
			prettyJson(w, err.Error())
		} else {
			prettyJson(w, "Calibration done")
		}
	} else {
		chanId, err := strconv.Atoi(channel[0])
		if err != nil {
			prettyJson(w, "channel invalid")
		}
		err = calibrationOne(chanId)
		if err != nil {
			prettyJson(w, err.Error())
		} else {
			prettyJson(w, "Calibration done")
		}
	}
}

func calibrationAll() error {
	var args []string
	args = append(args, "-s", caliSamples, "cf_axi_adc")
	for i := 0; i <= 6; i++ {
		args = append(args, fmt.Sprintf("voltage%d", i))
	}
	logrus.Info(args)

	cmd := exec.Command("iio_readdev", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	samplePoints := new(bytes.Buffer)
	cmd.Stdout = samplePoints

	if err := cmd.Start(); err != nil {
		logrus.Errorf("calibrationAll cmd.Start() failed: %s", err.Error())
		return err
	}
	logrus.Info("iio_readdev started")
	if err := cmd.Wait(); err != nil {
		logrus.Errorf("calibrationAll cmd.Wait() failed: %s", err.Error())
		return err
	}
	logrus.Info("iio_readdev wait done")

	logrus.Info(len(samplePoints.Bytes()))
	return nil
}

func calibrationOne(chanId int) error {
	var args []string
	if chanId < 0 || chanId > 14 {
		return fmt.Errorf("chanid=%d error", chanId)
	}
	var devName string
	if chanId < 7 {
		devName = "cf_axi_adc"
	} else {
		chanId -= 7
		devName = "cf_axi_adc_1"
	}
	args = append(args, "-s", caliSamples, devName, fmt.Sprintf("voltage%d", chanId))

	logrus.Info(devName, args)
	return nil
}

func prettyJson(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func inferWrapper(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var args []string

	ch := params.ByName("channel")

	id, err := strconv.Atoi(ch)
	logrus.Errorf("channel = %v, id = %d", ch, id)
	if err != nil {
		logrus.Errorf("channel = %v not allowed", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	args = append(args, "-i", ch, "-f", "/tmp/infer.png")
	logrus.Errorf("args = %v", args)
	cmd := exec.Command("/root/yolo3", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	jsonResult := new(bytes.Buffer)
	cmd.Stdout = jsonResult

	if err := cmd.Start(); err != nil {
		logrus.Errorf("inferWrapper cmd.Start() failed: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	logrus.Errorf("start done")
	err = cmd.Wait()
	if err != nil {
		logrus.Errorf("inferWrapper cmd.Wait() failed: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}
	logrus.Errorf("wait done")
	var v interface{}
	json.NewDecoder(jsonResult).Decode(&v)
	prettyJson(w, v)

}

func weightLoadWrapper(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var args []string

	args = append(args, params.ByName("channel"))

	cmd := exec.Command("./weightLoad/weightLoad", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	if err := cmd.Start(); err != nil {
		logrus.Errorf("weightLoadWrapper cmd.Start() failed: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err := cmd.Wait()
	if err != nil {
		logrus.Errorf("weightLoadWrapper cmd.Wait() failed: %s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)

}
