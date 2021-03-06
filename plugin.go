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
		Region:     aws.String(p.Region),
		MaxRetries: aws.Int(20),
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
			log.WithError(err).Error("Problem creating application version")

			if p.EnvironmentUpdate == false {
				return err
			}

			log.Warning("Ignoring error and attempting to update")
		}
	}

	if p.EnvironmentUpdate {

		err := waitEnvironmentToBeReady(
			client,
			p.Application,
			p.EnvironmentName,
			p.Timeout,
		)

		if err != nil {
			return err
		}

		appFields := log.WithFields(log.Fields{
			"application":  p.Application,
			"environment":  p.EnvironmentName,
			"versionlabel": p.VersionLabel,
			"timeout":      p.Timeout,
		})

		tick := time.Tick(time.Second * 10)
		tout := time.After(p.Timeout)

		description, err := client.UpdateEnvironment(
			&elasticbeanstalk.UpdateEnvironmentInput{
				VersionLabel:    aws.String(p.VersionLabel),
				ApplicationName: aws.String(p.Application),
				Description:     aws.String(p.Description),
				EnvironmentName: aws.String(p.EnvironmentName),
			},
		)

		appFields.Infoln(description)

		if err != nil {
			appFields.WithError(err).Error("Problem updating beanstalk")
			return err
		}

		appFields.Info("Waiting for environment to finish updating")

		for {
			select {

			case <-tick:

				envs, err := client.DescribeEnvironments(
					&elasticbeanstalk.DescribeEnvironmentsInput{
						ApplicationName:  aws.String(p.Application),
						EnvironmentNames: aws.StringSlice([]string{p.EnvironmentName}),
					},
				)

				if err != nil {
					appFields.WithError(err).Error("Problem retrieving environment information")
					return err
				}

				// get the latest event
				events, err := client.DescribeEvents(&elasticbeanstalk.DescribeEventsInput{
					ApplicationName: aws.String(p.Application),
					EnvironmentName: aws.String(p.EnvironmentName),
					MaxRecords:      aws.Int64(1),
				})

				if err != nil {
					appFields.WithError(err).Error("Problem retrieving environment events")
					return err
				}

				env := envs.Environments[0]

				event := aws.StringValue(events.Events[0].Message)
				status := aws.StringValue(env.Status)
				health := aws.StringValue(env.Health)
				version := aws.StringValue(env.VersionLabel)

				envFields := log.WithFields(log.Fields{
					"event":   event,
					"version": version,
					"status":  status,
					"health":  health,
				})

				envFields.Info("Updating")

				if status == elasticbeanstalk.EnvironmentStatusReady {

					if p.VersionLabel != version {
						err := errors.New("update did not finish")
						appFields.WithError(err).Error("Update failed, please check EB environment logs")
						return err
					}

					appFields.WithFields(log.Fields{
						"application":  p.Application,
						"environment":  p.EnvironmentName,
						"versionlabel": p.VersionLabel,
					}).Info("Update finished successfully")

					return nil
				}

				if status != elasticbeanstalk.EnvironmentStatusUpdating {
					err := errors.New("environment is not updating")
					appFields.WithError(err).Error("Update failed")
					return err
				}

			case <-tout:
				err := errors.New("timed out")

				if err != nil {
					appFields.WithError(err).Error("Environment failed to update")
					return err
				}

			}
		}
	}

	return nil
}

func waitEnvironmentToBeReady(client *elasticbeanstalk.ElasticBeanstalk, application string, environment string, timeout time.Duration) error {

	appFields := log.WithFields(log.Fields{
		"application": application,
		"environment": environment,
		"timeout":     timeout,
	})

	tick := time.Tick(time.Second * 10)
	tout := time.After(timeout)

	for {
		select {

		case <-tick:

			envs, err := client.DescribeEnvironments(
				&elasticbeanstalk.DescribeEnvironmentsInput{
					ApplicationName:  aws.String(application),
					EnvironmentNames: aws.StringSlice([]string{environment}),
				},
			)

			if err != nil {
				appFields.WithError(err).Error("Problem retrieving environment information")
				return err
			}

			env := envs.Environments[0]

			if aws.StringValue(env.Status) == elasticbeanstalk.EnvironmentStatusReady {
				return nil
			}

			appFields.WithField("status", aws.StringValue(env.Status)).Info("Waiting for environment to be ready")

		case <-tout:
			err := errors.New("timed out")
			appFields.WithError(err).Error("Environment never got into ready state")
			return err
		}
	}
}
