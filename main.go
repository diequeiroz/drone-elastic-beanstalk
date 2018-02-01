package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

var build string

func main() {
	app := cli.NewApp()
	app.Name = "Beanstalk deployment plugin"
	app.Usage = "beanstalk deployment plugin"
	app.Action = run
	app.Version = fmt.Sprintf("1.0.0+%s", build)
	app.Flags = []cli.Flag{

		cli.StringFlag{
			Name:   "access-key",
			Usage:  "aws access key",
			EnvVar: "PLUGIN_ACCESS_KEY,AWS_ACCESS_KEY_ID",
		},
		cli.StringFlag{
			Name:   "secret-key",
			Usage:  "aws secret key",
			EnvVar: "PLUGIN_SECRET_KEY,AWS_SECRET_ACCESS_KEY",
		},
		cli.StringFlag{
			Name:   "bucket",
			Usage:  "aws bucket",
			EnvVar: "PLUGIN_BUCKET",
		},
		cli.StringFlag{
			Name:   "region",
			Usage:  "aws region",
			Value:  "us-east-1",
			EnvVar: "PLUGIN_REGION",
		},
		cli.StringFlag{
			Name:   "bucket-key",
			Usage:  "upload files from source folder",
			EnvVar: "PLUGIN_BUCKET_KEY",
		},
		cli.StringFlag{
			Name:   "application",
			Usage:  "application name for beanstalk",
			EnvVar: "PLUGIN_APPLICATION",
		},
		cli.StringSliceFlag{
			Name:   "environments",
			Usage:  "environment name in the app to update",
			EnvVar: "PLUGIN_ENVIRONMENTS, PLUGIN_ENVIRONMENT_NAME",
		},
		cli.StringFlag{
			Name:   "version-label",
			Usage:  "version label for the app",
			EnvVar: "PLUGIN_VERSION_LABEL",
		},
		cli.StringFlag{
			Name:   "description",
			Usage:  "description for the app version",
			EnvVar: "PLUGIN_DESCRIPTION",
		},
		cli.StringFlag{
			Name:   "auto-create",
			Usage:  "auto create app if it doesn't exist",
			EnvVar: "PLUGIN_AUTO_CREATE",
		},
		cli.StringFlag{
			Name:   "process",
			Usage:  "Preprocess and validate manifest",
			EnvVar: "PLUGIN_PROCESS",
		},
		cli.StringFlag{
			Name:   "environment-update",
			Usage:  "update the environment",
			EnvVar: "PLUGIN_ENVIRONMENT_UPDATE",
		},
		cli.StringFlag{
			Name:   "timeout",
			Usage:  "deploy timeout in minutes",
			Value:  "20",
			EnvVar: "PLUGIN_TIMEOUT",
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
func run(c *cli.Context) error {

	timeout, err := strconv.Atoi(c.String("timeout"))

	if err != nil {
		log.WithFields(log.Fields{
			"timeout": c.String("timeout"),
			"error":   err,
		}).Error("invalid timeout configuration")
		return err
	}

	plugin := Plugin{
		Region:            c.String("region"),
		Key:               c.String("access-key"),
		Secret:            c.String("secret-key"),
		Bucket:            c.String("bucket"),
		BucketKey:         c.String("bucket-key"),
		Application:       c.String("application"),
		Environments:      c.StringSlice("environments"),
		VersionLabel:      c.String("version-label"),
		Description:       c.String("description"),
		AutoCreate:        c.Bool("auto-create"),
		Process:           c.Bool("process"),
		EnvironmentUpdate: c.Bool("environment-update"),
		Timeout:           time.Duration(timeout) * time.Minute,
	}

	return plugin.Exec()
}
