package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Secrets map[string]struct {
		Value map[string]string `yaml:"value"`
		Tags  map[string]string `yaml:"tags"`
	} `yaml:"secrets"`
}

type CLIOpts struct {
	Profile string
	Config  string
	Region  string
	KMSKey  string
}

func main() {

	cli := parseFlags()

	data, err := ioutil.ReadFile(cli.Config)
	if err != nil {
		log.Printf("Failed to read config: %v", err)
	}

	var config Config

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Failed to parse config: %v", err)
	}

	sess, err := createAWSSession(cli.Profile, cli.Region)

	svc := secretsmanager.New(sess)

	manageSecrets(svc, config, &cli.KMSKey)

}

func parseFlags() CLIOpts {
	profile := flag.String("profile", "default", "AWS profile to use")
	configPath := flag.String("config", "", "Path to the config file")
	region := flag.String("region", "us-east-1", "AWS region")
	kms := flag.String("kms", "", "KMS key ID or alias to use for encrypting the secrets")

	flag.Parse()

	return CLIOpts{
		Profile: *profile,
		Config:  *configPath,
		Region:  *region,
		KMSKey:  *kms,
	}
}

func createAWSSession(profile, region string) (*session.Session, error) {
	sessOpts := session.Options{
		Profile: profile,
		Config: aws.Config{
			Region: aws.String(region),
		},
	}

	return session.NewSessionWithOptions(sessOpts)
}

func manageSecrets(svc *secretsmanager.SecretsManager, config Config, kms *string) {
	for name, secret := range config.Secrets {
		secretValue, err := json.Marshal(secret.Value)
		if err != nil {
			log.Printf("Failed to marshal secret %s: %v", name, err)
			continue
		}

		var tags []*secretsmanager.Tag
		for k, v := range secret.Tags {
			tags = append(tags, &secretsmanager.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}

		// this is to avoid updating the secret if the value is the same
		currentValue, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
			SecretId: aws.String(name),
		})

		switch {
		case err == nil:
			// If current value is the same as new value, skip update
			if currentValue.SecretString != nil && *currentValue.SecretString == string(secretValue) {
				fmt.Printf("Secret %s has no changes, skipping update\n", name)
				continue
			}

			updateInput := &secretsmanager.UpdateSecretInput{
				SecretId:     aws.String(name),
				SecretString: aws.String(string(secretValue)),
				KmsKeyId:     kms,
			}

			if *kms != "" {
				updateInput.KmsKeyId = kms
			}

			_, err = svc.UpdateSecret(updateInput)
			if err != nil {
				log.Printf("Failed to update secret %s: %v", name, err)
			} else {
				fmt.Printf("Secret %s updated successfully\n", name)
			}

		case isAWSError(err, secretsmanager.ErrCodeResourceNotFoundException):
			createInput := &secretsmanager.CreateSecretInput{
				Name:         aws.String(name),
				SecretString: aws.String(string(secretValue)),
				KmsKeyId:     kms,
				Tags:         tags,
			}

			if *kms != "" {
				createInput.KmsKeyId = kms
			}

			_, err := svc.CreateSecret(createInput)
			if err != nil {
				log.Printf("Failed to create secret %s: %v", name, err)
			} else {
				fmt.Printf("Secret %s created successfully\n", name)
			}
		default:
			log.Printf("Failed to describe secret %s: %v", name, err)
		}
	}
}

func isAWSError(err error, code string) bool {
	if aerr, ok := err.(awserr.Error); ok {
		if aerr.Code() == code {
			return true
		}
	}
	return false
}
