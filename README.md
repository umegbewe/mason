## Mason

I have been working with lots of secrets in AWS Secrets Manager the past few days, using the CLI and remembering ``aws`` commands is daunting, the console is worse and feels unnatural


I built ``mason`` to make stuff easier as simple as defining secrets this way

```yaml
secrets:
   # create key value secret
  /example/postgres/secret:
    key_value:
      POSTGRES_USERNAME: postgres
      POSTGRES_PASSWORD: password
    tags:
      environment: local
  # create plain text secret
  /example/nas/secret-lyric:
    plaintext: |
      Early on the wakeup
      Cunning as the father of the twelve sons of jacob
      Shake up the world cake crumbs
    tags:
     track: fever
  # create file secret
  /example/file/secret:
    file: "example.json"
    tags:
      file-type: json
```

## **Install**

- Download [binaries](https://github.com/umegbewe/mason/releases)

- Go install:
```
go install github.com/umegbewe/mason@latest
```

## **Usage**

```text
Usage:

    mason [OPTIONS]

Options:

    -profile: AWS profile to use, default is  "default"

    -config: Path to where config for secrets is defined, see secrets.yml for example.

    -region: The region where the secrets should be created. Default is "us-east-1".'

    -kms: KMS key ID or alias to use for encrypting the secrets
```

## **License**

This tool is released under the MIT License. See **[LICENSE](https://github.com/umegbewe/mason/blob/main/LICENSE)** for more information.