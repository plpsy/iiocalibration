package main

import (
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"github.com/plpsy/iiocalibration/api"
	"github.com/plpsy/iiocalibration/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const ()

var (
	globalFlags = []cli.Flag{
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "debug mode",
			EnvVar: "DEBUG",
		},

		cli.StringFlag{
			Name:   "listen",
			Usage:  "The address that the listens on",
			EnvVar: "LISTEN_ADDR",
			Value:  ":80",
		},
	}

	cmdServer = cli.Command{
		Name:   "server",
		Usage:  "start the flyfront",
		Flags:  globalFlags,
		Action: actionServer,
	}

	cmdVersion = cli.Command{
		Name:      "version",
		ShortName: "v",
		Usage:     "print version",
		Action: func(c *cli.Context) {
			version.Info().WriteTo(os.Stdout)
		},
	}

	cmds = []cli.Command{cmdServer, cmdVersion}
)

func main() {
	app := cli.NewApp()
	app.Name = "iiocalibration"
	app.Usage = "adc calibration"
	app.Version = version.GetVersion()

	if gitCommit := version.GetGitCommit(); gitCommit != "" {
		app.Version += "-" + gitCommit
	}

	api.LoadAndSetOffset()

	app.Author = "panling"
	app.Email = "panling@aiclab.org"
	app.Flags = globalFlags
	app.Before = func(ctx *cli.Context) error {
		if ctx.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logrus.SetLevel(logrus.InfoLevel)
		}
		return nil
	}
	app.Commands = cmds
	app.RunAndExitOnError()
}

func actionServer(ctx *cli.Context) {

	r := RegisterHandler()
	addr := ctx.GlobalString("listen")
	if addr == "" {
		addr = ":80"
	}
	logrus.Info("listen on addr: ", addr)
	err := http.ListenAndServe(addr, r) //设置监听的端口
	if err != nil {
		logrus.Fatal("ListenAndServe: ", err)
	}
}

func RegisterHandler() *httprouter.Router {
	router := httprouter.New()
	router.GET("/params", api.CalibrationParams)
	router.GET("/regparams", api.GetRegsParams)
	router.DELETE("/regparams", api.ClearRegsParams)
	router.POST("/calibration", api.Calibration)
	router.POST("/reboot", api.RestartSystem)
	return router
}
