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
		KeyValue  map[string]string `yaml:"key_value,omitempty"`
		PlainText string            `yaml:"plaintext,omitempty"`
		File      string            `yaml:"file,omitempty"`
		Tags      map[string]string `yaml:"tags"`
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
		log.Fatalf("Failed to read config: %v", err)
	}

	var config Config

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Failed to parse config: %v", err)
	}

	err = validateConfig(config)
	if err != nil {
		log.Fatalf("Invalid config: %v", err)
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

func validateConfig(config Config) error {
	for name, secret := range config.Secrets {
		if secret.KeyValue != nil && secret.File != "" {
			return fmt.Errorf("secret '%s' has both KeyValue and File set, which is not allowed", name)
		}

		if secret.KeyValue == nil && secret.File == "" && secret.PlainText == "" {
			return fmt.Errorf("secret '%s' must have either KeyValue, File, or PlainText set", name)
		}

		for tagKey, tagValue := range secret.Tags {
			if tagKey == "" || tagValue == "" {
				return fmt.Errorf("secret '%s' has invalid tag. Tags must not be empty", name)
			}
		}
	}
	return nil
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
		var secretValue string

		if secret.KeyValue != nil {
			marshaledValue, err := json.Marshal(secret.KeyValue)
			if err != nil {
				log.Printf("Failed to marshal secret %s: %v", name, err)
				continue
			}
			secretValue = string(marshaledValue)
		} else if secret.File != "" {
			content, err := ioutil.ReadFile(secret.File)
			if err != nil {
				log.Printf("Failed to read file %s: %v", secret.File, err)
				continue
			}
			secretValue = string(content)
		} else {
			secretValue = secret.PlainText
		}

		tags := make([]*secretsmanager.Tag, 0, len(secret.Tags))
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
