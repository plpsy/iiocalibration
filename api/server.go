package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

const (
	cfgFilePath = "/media/sd-mmcblk1p2/calibration.json"
	caliSamples = 1024
)

func CalibrationParams(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var caliparams map[string]map[int]int32
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

func GetRegsParams(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	caliParams, err := getOffsetRegs()
	if err != nil {
		writeResponse(w, err.Error())
		return
	}
	writeResponse(w, caliParams)
}

func ClearRegsParams(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	err := clearOffsetRegs()
	if err != nil {
		writeResponse(w, err.Error())
		return
	}
	writeResponse(w, "ClearRegsParams done")
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
	if err != nil {
		return err
	}
	err = calibration("cf_axi_adc_1", []int{0, 1, 2, 3, 4, 5, 6, 7})
	return err
}

func LoadAndSetOffset() {
	var caliparams map[string]map[int]int32
	fp, err := os.Open(cfgFilePath)
	if err != nil {
		logrus.Error("loadAndSetOffset open config error", err)
		return
	}
	defer fp.Close()
	err = json.NewDecoder(fp).Decode(&caliparams)
	if err != nil {
		logrus.Error("loadAndSetOffset decode config error", err)
		return
	}

	err = setOffsetRegs(caliparams)
	if err != nil {
		logrus.Error("LoadAndSetOffset setOffsetRegs error", err)
	}
	logrus.Info("LoadAndSetOffset done")
}

func setOffsetRegs(params map[string]map[int]int32) error {
	for devName, devParams := range params {
		for chanId, offset := range devParams {
			err := setDevOffset(devName, chanId, offset)
			if err != nil {
				logrus.Error("setOffsetRegs setDevOffset error", err)
				return err
			}
		}
	}
	return nil
}

func clearOffsetRegs() error {
	params := make(map[string]map[int]int32)
	devName := "cf_axi_adc"
	params[devName] = make(map[int]int32)
	for i := 0; i < 7; i++ {
		params[devName][i] = 0
	}
	devName = "cf_axi_adc_1"
	params[devName] = make(map[int]int32)
	for i := 0; i < 8; i++ {
		params[devName][i] = 0
	}
	return setOffsetRegs(params)
}

func getOffsetRegs() (map[string]map[int]int32, error) {
	params := make(map[string]map[int]int32)
	devName := "cf_axi_adc"
	params[devName] = make(map[int]int32)
	for i := 0; i < 7; i++ {
		offset, err := getDevOffset(devName, i)
		if err != nil {
			logrus.Errorf("getOffsetRegs %s chanid=%d err", devName, i, err)
			return nil, err
		}
		params[devName][i] = offset
	}

	devName = "cf_axi_adc_1"
	params[devName] = make(map[int]int32)
	for i := 0; i < 8; i++ {
		offset, err := getDevOffset(devName, i)
		if err != nil {
			logrus.Errorf("getOffsetRegs %s chanid=%d err", devName, i, err)
			return nil, err
		}
		params[devName][i] = offset
	}
	return params, nil
}

func calibration(devName string, chanIds []int) error {
	var args []string
	args = append(args, "-s", fmt.Sprintf("%d", caliSamples), devName)
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

	averages := calcAverage(samplePoints.Bytes(), len(chanIds))
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
		err = saveAverage(devName, id, averages[i])
		if err != nil {
			err1 := fmt.Errorf("calibration saveAverage(%s) chanid(%d) failed: %s", devName, id, err.Error())
			logrus.Errorf(err1.Error())
			return err1
		}
	}
	return nil
}

func saveAverage(devName string, chanId int, offset int32) error {
	var params map[string]map[int]int32
	var devParams map[int]int32

	// 先读出已有配置,再创建新文件
	fp, err := os.Open(cfgFilePath)
	if err == nil {
		err = json.NewDecoder(fp).Decode(&params)
		if err != nil {
			params = nil
		}
		fp.Close()
	}

	fp, err = os.Create(cfgFilePath)
	if err != nil {
		return err
	}

	defer fp.Close()
	if params == nil {
		params = make(map[string]map[int]int32)
	}
	devParams, ok := params[devName]
	if ok {
		devParams[chanId] = offset
	} else {
		devParams := make(map[int]int32)
		devParams[chanId] = offset
		params[devName] = devParams
	}

	err = json.NewEncoder(fp).Encode(params)
	if err != nil {
		return fmt.Errorf("write json config err=%v", err)
	}
	return nil
}

func calcAverage(points []byte, chanNum int) []int32 {
	samples := make([]int32, caliSamples)
	averages := make([]int32, chanNum)

	for idx := 0; idx < chanNum; idx++ {
		for i := 0; i < caliSamples; i++ {
			// 找到采样点偏移
			off := i*chanNum*4 + idx*4
			bytebuff := bytes.NewBuffer(points[off : off+4])
			binary.Read(bytebuff, binary.LittleEndian, &samples[i])
			// 24位补码表示的有符号采样值,转换为32位有符号整数
			samples[i] <<= 8
			samples[i] >>= 8
		}
		logrus.Info(samples[0:64])
		var sum int64 = 0
		for i := 0; i < caliSamples; i++ {
			sum += (int64)(samples[i])
		}
		averages[idx] = (int32)(sum / (int64)(caliSamples))
		averages[idx] = (int32)((float32)(averages[idx]) * 0.75)
		averages[idx] = 0 - averages[idx]
	}

	return averages
}

func getDevOffset(devName string, chanId int) (offset int32, err error) {
	var msb, mib, lsb uint8

	off := 0x1E + chanId*3
	msb, err = readDevReg(devName, off)
	if err != nil {
		return
	}
	off += 1
	mib, err = readDevReg(devName, off)
	if err != nil {
		return
	}
	off += 1
	lsb, err = readDevReg(devName, off)
	if err != nil {
		return
	}
	offset = ((int32)(msb) << 16) | ((int32)(mib) << 8) | (int32)(lsb)
	offset <<= 8
	offset >>= 8

	return
}

func readDevReg(devName string, off int) (val uint8, err error) {
	var args []string

	args = append(args, devName, fmt.Sprintf("0x%02x", off))
	logrus.Info("readDevReg args:", args)

	cmd := exec.Command("iio_reg", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	valBuff := new(bytes.Buffer)
	cmd.Stdout = valBuff

	if err = cmd.Start(); err != nil {
		err = fmt.Errorf("readDevReg cmd.Start() failed: %s", err.Error())
		logrus.Errorf(err.Error())
		return
	}

	if err = cmd.Wait(); err != nil {
		err = fmt.Errorf("writeDevReg cmd.Wait() failed: %s", err.Error())
		logrus.Errorf(err.Error())
		return
	}
	varStr := strings.Replace(string(valBuff.Bytes()), "\n", "", -1)
	val64, err := strconv.ParseInt(varStr, 0, 64)
	if err != nil {
		err = fmt.Errorf("readDevReg strconv.ParseInt failed: %s", err.Error())
		logrus.Errorf(err.Error())
		return
	}
	val = (uint8)(val64)
	return
}

func writeDevReg(devName string, off int, val uint8) error {
	var args []string

	args = append(args, devName, fmt.Sprintf("0x%02x", off), fmt.Sprintf("%d", val))
	logrus.Info("writeDevReg args:", args)

	cmd := exec.Command("iio_reg", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}

	if err := cmd.Start(); err != nil {
		err1 := fmt.Errorf("writeDevReg cmd.Start() failed: %s", err.Error())
		logrus.Errorf(err1.Error())
		return err1
	}

	if err := cmd.Wait(); err != nil {
		err1 := fmt.Errorf("writeDevReg cmd.Wait() failed: %s", err.Error())
		logrus.Errorf(err1.Error())
		return err1
	}
	logrus.Info("writeDevReg wait done")
	return nil
}

func setDevOffset(devName string, chanId int, offset int32) error {
	var msb, mib, lsb uint8

	msb = (uint8)(offset >> 16)
	if offset < 0 {
		msb |= 0x80
	}
	mib = (uint8)(offset >> 8)
	lsb = (uint8)(offset)

	logrus.Infof("setDevOffset chanId=%d, offset=%d, msb/mib/lsb=(%02x/%02x/%02x)", chanId, offset, msb, mib, lsb)
	off := 0x1E + chanId*3
	err := writeDevReg(devName, off, msb)
	if err != nil {
		return err
	}
	off += 1
	err = writeDevReg(devName, off, mib)
	if err != nil {
		return err
	}
	off += 1
	err = writeDevReg(devName, off, lsb)
	if err != nil {
		return err
	}
	return nil
}

func calibrationOne(chanId int) error {
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
	logrus.Info("calibrationOne:", devName, chanId)
	err := calibration(devName, []int{chanId})
	return err
}

func prettyJson(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}
