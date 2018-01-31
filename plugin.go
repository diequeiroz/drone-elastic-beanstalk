package main

import (
	"errors"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elasticbeanstalk"
)

// Plugin defines the beanstalk plugin parameters.
type Plugin struct {
	Key    string
	Secret string
	Bucket string

	// us-east-1
	// us-west-1
	// us-west-2
	// eu-west-1
	// ap-southeast-1
	// ap-southeast-2
	// ap-northeast-1
	// sa-east-1
	Region string

	BucketKey         string
	Application       string
	EnvironmentName   string
	VersionLabel      string
	Description       string
	AutoCreate        bool
	Process           bool
	EnvironmentUpdate bool

	Timeout time.Duration
}

// Exec runs the plugin
func (p *Plugin) Exec() error {
	// create the client

	conf := &aws.Config{
		Region: aws.String(p.Region),
	}

	log.WithFields(log.Fields{
		"region":       p.Region,
		"application":  p.Application,
		"environment":  p.EnvironmentName,
		"bucket":       p.Bucket,
		"bucket-key":   p.BucketKey,
		"versionlabel": p.VersionLabel,
		"description":  p.Description,
		"env-update":   p.EnvironmentUpdate,
		"auto-create":  p.AutoCreate,
		"timeout":      p.Timeout,
	}).Info("Authenticating")

	// Use key and secret if provided otherwise fall back to ec2 instance profile
	if p.Key != "" && p.Secret != "" {
		log.Warning("AWS Key and Secret not found, will attempt to use IAM role")
		conf.Credentials = credentials.NewStaticCredentials(p.Key, p.Secret, "")
	}
	client := elasticbeanstalk.New(session.New(), conf)

	if p.Bucket != "" && p.BucketKey != "" {

		log.WithFields(log.Fields{
			"application":  p.Application,
			"bucket":       p.Bucket,
			"bucket-key":   p.BucketKey,
			"versionlabel": p.VersionLabel,
			"description":  p.Description,
			"auto-create":  p.AutoCreate,
		}).Info("Attempting to create application version")

		_, err := client.CreateApplicationVersion(
			&elasticbeanstalk.CreateApplicationVersionInput{
				VersionLabel:          aws.String(p.VersionLabel),
				ApplicationName:       aws.String(p.Application),
				Description:           aws.String(p.Description),
				AutoCreateApplication: aws.Bool(p.AutoCreate),
				Process:               aws.Bool(p.Process),
				SourceBundle: &elasticbeanstalk.S3Location{
					S3Bucket: aws.String(p.Bucket),
					S3Key:    aws.String(p.BucketKey),
				},
			},
		)

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Problem creating application")
			return err
		}
	}

	if p.EnvironmentUpdate {

		ctx := log.WithFields(log.Fields{
			"application":  p.Application,
			"environment":  p.EnvironmentName,
			"versionlabel": p.VersionLabel,
			"timeout":      p.Timeout,
		})

		ctx.Info("Attempting to update environment")

		_, err := client.UpdateEnvironment(
			&elasticbeanstalk.UpdateEnvironmentInput{
				VersionLabel:    aws.String(p.VersionLabel),
				ApplicationName: aws.String(p.Application),
				Description:     aws.String(p.Description),
				EnvironmentName: aws.String(p.EnvironmentName),
			},
		)

		if err != nil {
			ctx.WithFields(log.Fields{
				"error": err,
			}).Error("Problem updating beanstalk")
			return err
		}

		ctx.Info("Wating for environment to finish updating")

		tick := time.Tick(time.Second * 10)
		timeout := time.After(p.Timeout)

		for {
			select {

			case <-tick:

				envs, err := client.DescribeEnvironments(
					&elasticbeanstalk.DescribeEnvironmentsInput{
						ApplicationName:  aws.String(p.Application),
						EnvironmentNames: []*string{aws.String(p.EnvironmentName)},
					},
				)

				if err != nil {
					ctx.WithFields(log.Fields{
						"error": err,
					}).Error("Problem retrieving environment information")
					return err
				}

				env := envs.Environments[0]

				status := aws.StringValue(env.Status)
				health := aws.StringValue(env.Health)
				version := aws.StringValue(env.VersionLabel)

				// get the latest event
				event, err := client.DescribeEvents(&elasticbeanstalk.DescribeEventsInput{
					ApplicationName: aws.String(p.Application),
					EnvironmentName: aws.String(p.EnvironmentName),
					MaxRecords:      aws.Int64(1),
				})

				if err != nil {
					ctx.WithFields(log.Fields{
						"error": err,
					}).Error("Problem retrieving environment events")
					return err
				}

				if status == elasticbeanstalk.EnvironmentStatusReady {

					if p.VersionLabel != version {
						err := errors.New("version mismatch")
						ctx.WithFields(log.Fields{
							"err":             err,
							"current-version": version,
							"status":          status,
							"health":          health,
							"event":           event.Events[0],
						}).Error("Update failed")
						return err
					}
				}

				if status != elasticbeanstalk.EnvironmentStatusUpdating {
					err := errors.New("environment is not updating")
					ctx.WithFields(log.Fields{
						"err":             err,
						"current-version": version,
						"status":          status,
						"health":          health,
						"event":           event.Events[0],
					}).Error("Update failed")
					return err
				}

				ctx.WithFields(log.Fields{
					"current-version": version,
					"status":          status,
					"health":          health,
				}).Info("Updating")

			case <-timeout:
				err := errors.New("timed out")

				if err != nil {
					ctx.WithFields(log.Fields{
						"error": err,
					}).Error("Environment failed to update")
					return err
				}

			}
		}
	}

	log.WithFields(log.Fields{
		"application":  p.Application,
		"environment":  p.EnvironmentName,
		"versionlabel": p.VersionLabel,
	}).Info("Update finished successfully")

	return nil
}
