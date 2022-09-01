package api

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func CalibrationParams(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var caliparams map[string]map[int]int
	fp, err := os.Open(cfgFilePath)
	if err != nil {
		writeResponse(w, caliparams)
		return
	}
	defer fp.Close()
	err = json.NewDecoder(fp).Decode(&caliparams)
	if err != nil {
		writeResponse(w, "read caliparams config file error")
	}
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
	err := calibration("cf_axi_adc", []int{0, 1, 2, 3, 4, 5, 6})
	return err
}

func calibration(devName string, chanIds []int) error {
	var args []string
	args = append(args, "-s", caliSamples, devName)
	for _, id := range chanIds {
		args = append(args, fmt.Sprintf("voltage%d", id))
	}
	logrus.Info(args)

	cmd := exec.Command("iio_readdev", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	samplePoints := new(bytes.Buffer)
	cmd.Stdout = samplePoints

	if err := cmd.Start(); err != nil {
		err1 := fmt.Errorf("calibrationAll cmd.Start() failed: %s", err.Error())
		logrus.Errorf(err1.Error())
		return err1
	}
	logrus.Info("iio_readdev started")
	if err := cmd.Wait(); err != nil {
		err1 := fmt.Errorf("calibration cmd.Wait() failed: %s", err.Error())
		logrus.Errorf(err1.Error())
		return err1
	}
	logrus.Info("iio_readdev wait done")

	var averages []int
	if err := calcAverage(samplePoints.Bytes(), chanIds, averages); err != nil {
		err1 := fmt.Errorf("calibration calcAverage failed: %s", err.Error())
		logrus.Errorf(err1.Error())
		return err1
	}
	if len(averages) != len(chanIds) {
		err1 := fmt.Errorf("calibration calcAverage, len(averages)[%+v] != len(chanIds)[%+v]", averages, chanIds)
		logrus.Errorf(err1.Error())
		return err1
	}

	for i, id := range chanIds {
		err := setDevOffset(devName, id, averages[i])
		if err != nil {
			err1 := fmt.Errorf("calibration setDevOffset(%s) chanid(%d) failed: %s", devName, id, err.Error())
			logrus.Errorf(err1.Error())
			return err1
		}
	}
	return nil
}

func saveAverage(devName string, chanId int, offset int) error {
	var params map[string]map[int]int
	var devParams map[int]int
	fp, err := os.OpenFile(cfgFilePath, os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer fp.Close()
	err = json.NewDecoder(fp).Decode(&params)
	if err != nil {
		return fmt.Errorf("read json config err=%v", err)
	}
	devParams, ok := params[devName]
	if ok {
		devParams[chanId] = offset
	} else {
		devParams := make(map[int]int)
		devParams[chanId] = offset
		params[devName] = devParams
	}

	err = json.NewEncoder(fp).Encode(params)
	if err != nil {
		return fmt.Errorf("write json config err=%v", err)
	}
	return nil
}

func calcAverage(points []byte, chanIds []int, averages []int) error {

	for _, id := range chanIds {
		averages = append(averages, id)
	}
	return nil
}

func setDevOffset(devName string, chanId int, offset int) error {
	err := saveAverage(devName, chanId, offset)
	return err
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
