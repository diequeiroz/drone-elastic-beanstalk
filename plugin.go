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

	BucketKey    string
	Application  string
	Environments []string

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
		"environments": p.Environments,
		"bucket":       p.Bucket,
		"bucket-key":   p.BucketKey,
		"versionlabel": p.VersionLabel,
		"description":  p.Description,
		"env-update":   p.EnvironmentUpdate,
		"auto-create":  p.AutoCreate,
		"timeout":      p.Timeout,
	}).Info("Authenticating")

	if p.Key != "" && p.Secret != "" {
		conf.Credentials = credentials.NewStaticCredentials(p.Key, p.Secret, "")
	} else {
		log.Warn("AWS Key and/or Secret not provided (falling back to ec2 instance profile)")
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
		}).Info("Creating application version")

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

	ctx := log.WithFields(log.Fields{
		"application":  p.Application,
		"environments": p.Environments,
		"versionlabel": p.VersionLabel,
		"timeout":      p.Timeout,
	})

	if p.EnvironmentUpdate {

		ctx.Info("Updating environment")

		for _, environmentName := range p.Environments {
			_, err := client.UpdateEnvironment(
				&elasticbeanstalk.UpdateEnvironmentInput{
					VersionLabel:    aws.String(p.VersionLabel),
					ApplicationName: aws.String(p.Application),
					Description:     aws.String(p.Description),
					EnvironmentName: aws.String(environmentName),
				},
			)

			if err != nil {
				ctx.WithFields(log.Fields{
					"error": err,
				}).Error("Problem updating beanstalk")
				return err
			}
		}

		ctx.Info("Waiting for environment to finish updating")

		tick := time.Tick(time.Second * 10)
		timeout := time.After(p.Timeout)

		for {
			select {

			case <-tick:

				envs, err := client.DescribeEnvironments(
					&elasticbeanstalk.DescribeEnvironmentsInput{
						ApplicationName:  aws.String(p.Application),
						EnvironmentNames: aws.StringSlice(p.Environments),
					},
				)

				if err != nil {
					log.WithFields(log.Fields{
						"error": err,
					}).Error("Problem retrieving environment information")
					return err
				}

				for _, env := range envs.Environments {

					status := aws.StringValue(env.Status)
					health := aws.StringValue(env.Health)
					version := aws.StringValue(env.VersionLabel)

					// get the latest event
					event, err := client.DescribeEvents(&elasticbeanstalk.DescribeEventsInput{
						ApplicationName: aws.String(p.Application),
						EnvironmentName: env.EnvironmentName,
						MaxRecords:      aws.Int64(1),
					})

					if err != nil {
						log.WithFields(log.Fields{
							"error":       err,
							"environment": env.EnvironmentName,
						}).Error("Problem retrieving environment events")
						return err
					}

					if status == elasticbeanstalk.EnvironmentStatusReady {

						if p.VersionLabel != version {
							err := errors.New("version mismatch")
							log.WithFields(log.Fields{
								"err":             err,
								"current-version": version,
								"status":          status,
								"health":          health,
								"event":           event.Events[0],
								"environment":     env.EnvironmentName,
							}).Error("Update failed")
							return err
						}
					}

					if status != elasticbeanstalk.EnvironmentStatusUpdating {
						err := errors.New("environment is not updating")
						log.WithFields(log.Fields{
							"err":             err,
							"current-version": version,
							"status":          status,
							"health":          health,
							"event":           event.Events[0],
							"environment":     env.EnvironmentName,
						}).Error("Update failed")
						return err
					}

					log.WithFields(log.Fields{
						"environment":     env.EnvironmentName,
						"current-version": version,
						"status":          status,
						"health":          health,
					}).Info("Updating")
				}

			case <-timeout:
				err := errors.New("timed out")

				if err != nil {
					ctx.WithFields(log.Fields{
						"error": err,
					}).Error("Environments failed to update")
					return err
				}

			}
		}
	}

	ctx.WithFields(log.Fields{
		"application":  p.Application,
		"environments": p.Environments,
		"versionlabel": p.VersionLabel,
	}).Info("Update finished successfully")

	return nil
}
