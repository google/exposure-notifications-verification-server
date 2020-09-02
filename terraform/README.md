# Provisioning the verification server

This is a set of Terraform configurations which create the required
infrastructure for a verification server on Google Cloud. Please note that
**Terraform is only used for the initial deployment and provisioning of
underlying infrastructure!** It is not used for continuous delivery or
continuous deployment.

## Requirements

- Terraform 0.13.1 [Installation guide](https://www.terraform.io/downloads.html)

- firebase-cli. [Installation guide](https://firebase.google.com/docs/cli)

- gcloud. [Installation guide](https://cloud.google.com/sdk/install)

    Note: Make sure you **unset** `GOOGLE_APPLICATION_CREDENTIALS` in your
    environment:

    ```text
    unset GOOGLE_APPLICATION_CREDENTIALS
    ```

## Instructions

1.  Create a GCP project.
    [Instructions](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    Enable a billing account for this project, and note its project ID (the
    unique, unchangeable string that you will be asked for during creation):

    ```text
    $ export PROJECT_ID="<value-from-above>"
    ```

1.  Authenticate to gcloud with:

    ```text
    $ gcloud auth login
    ```

    This will open a web browser. Choose the right Google account and click
    "allow".

    ```text
    $ gcloud auth application-default login
    ```

    Set the quota project:

    ```text
    $ gcloud auth application-default set-quota-project "${PROJECT_ID}"
    ```

    This will open a web browser. Choose the right Google account and click
    "allow". Yes, this is nearly identical to the previous step.

    ```text
    $ firebase login
    ```

### Quick setup

This is for a POC. You should **not** use this method for production.

1.  Change into the `terraform/` directory. All future commands are run from the
    `terraform/` directory:

    ```text
    $ cd terraform/
    ```

1.  Save the project ID as a Terraform variable:

    ```text
    $ echo "project = \"${PROJECT_ID}\"" >> ./terraform.tfvars
    ```

1.  Run `terraform init`. Terraform will automatically download the plugins
    required to execute this code. You only need to do this once per machine.

    ```text
    $ terraform init
    ```

1.  Execute Terraform:

    ```text
    $ terraform apply
    ```

1.  After the initial provision, go to the Firebase admin console and enable
    your desired login (Facebook, email/password, etc).

### Production setup

For a production setup, create a new repo and import these configurations as a
Terraform module.

1.  Create a Cloud Storage bucket for storing remote state. This is important if
    you plan to have multiple people running Terraform or collaborating.

    ```text
    $ gsutil mb -p ${PROJECT_ID} gs://${PROJECT_ID}-terraform
    ```

1.  Create a new source control repository dedicated to managing infrastructure.
    This example assumes the repo is named `"en-infra"`.

1.  Create a definition to import this module inside your `"en-infra"` repo:

    ```text
    $ mkdir ${PROJECT_ID}
    ```


    ```text
    $ echo > ${PROJECT_ID}/main.tf <<EOF
      terraform {
        backend "gcs" {
          bucket = "${PROJECT_ID}-terraform"
        }
      }

      module "en" {
        source = "github.com/google/exposure-notifications-verification-server/terraform"

        project = var.project

        create_env_file = true

        service_environment = {
          server = {
            FIREBASE_PRIVACY_POLICY_URL   = "TODO"
            FIREBASE_TERMS_OF_SERVICE_URL = "TODO"
          }
        }
      }

      output "en" {
        value = module.en
      }
    EOF
    ```

    As shown above, all the variables defined in the Terraform are available as
    inputs/parameters to the module definition. See the `variables.tf` file for
    the full list of configuration options.

1.  Run `terraform init`. Terraform will automatically download the plugins
    required to execute this code. You only need to do this once per machine.

    ```text
    $ terraform init
    ```

1.  Execute Terraform:

    ```text
    $ terraform apply
    ```

1.  After the initial provision, go to the Firebase admin console and enable
    your desired login (Facebook, email/password, etc).

Terraform will create the required infrastructure including the database,
service accounts, keys, and secrets. **As a one-time operation**, Terraform will
also migrate the database schema and build/deploy the initial set of services on
Cloud Run. Terraform does not manage the lifecycle of those resources beyond
their initial creation.

### Local development and testing example deployment

The default Terraform deployment is a production-ready, high traffic deployment.
For local development and testing, you may want to use a less powerful setup:

```hcl
# terraform/terraform.tfvars
project                  = "..."
database_tier            = "db-custom-1-3840"
database_disk_size_gb    = 16
database_max_connections = 256
```


### Debugging

#### Cannot find firebase provider

If you're getting an error like:

```text
To work with <resource> its original provider configuration at
provider["registry.terraform.io/-/google"] is required, but it has been removed.
This occurs when a provider configuration is removed while objects created by
that provider still exist in the state. Re-add the provider configuration to
destroy <resource>, after which you can remove the provider configuration again.
```

It means you're upgrading from an older Terraform configuration. Try the following:

```text
$ terraform state rm google_project_iam_member.firebase
$ terraform state rm google_service_account.firebase
$ terraform state rm google_service_account_key.firebase
```

#### Firebase Tos Not Accepted

If you're getting an error like:

```text
Error: Error creating Project: googleapi: Error 403: Firebase Tos Not Accepted
```

It means the user hasn't accepted ToS(Terms of Services) of Firebase yet. Do:

1. Open a browser, navigate to https://firebase.google.com
1. Click `Add a project`, it will prompt Terms of Service agreement, agree
