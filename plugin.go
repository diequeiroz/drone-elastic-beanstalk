package main

import (
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
	Environment       string
	VersionLabel      string
	Description       string
	AutoCreate        bool
	Process           bool
	EnvironmentUpdate bool
	Debug             bool
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
		"environment":  p.Environment,
		"bucket":       p.Bucket,
		"bucket-key":   p.BucketKey,
		"versionlabel": p.VersionLabel,
		"description":  p.Description,
		"env-update":   p.EnvironmentUpdate,
		"auto-create":  p.AutoCreate,
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

		log.WithFields(log.Fields{
			"application":  p.Application,
			"environment":  p.Environment,
			"versionlabel": p.VersionLabel,
		}).Info("Attempting to update environment")

		_, err := client.UpdateEnvironment(
			&elasticbeanstalk.UpdateEnvironmentInput{
				VersionLabel:    aws.String(p.VersionLabel),
				ApplicationName: aws.String(p.Application),
				Description:     aws.String(p.Description),
				EnvironmentName: aws.String(p.Environment),
			},
		)

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Problem updating beanstalk")
			return err
		}
	}

	log.WithFields(log.Fields{
		"application":  p.Application,
		"environment":  p.Environment,
		"versionlabel": p.VersionLabel,
	}).Info("Update finished  successfully")

	return nil
}
