# S3 Photo Uploader

Web app for uploading photos to S3 with a per-upload storage class choice
(Standard or Glacier Deep Archive). After each upload the page shows the
photo's EXIF camera data, a GPS location map (if the photo has coordinates),
and its dominant color palette.

## Prerequisites

- Go 1.22+ (`brew install go`)
- An AWS account with the bucket + IAM resources provisioned (see
  *Infrastructure* below)
- AWS CLI authenticated (`aws login` or any credential method)

## Configuration

Copy the example env file and fill in your values:

```bash
cp .env.example .env
```

| Variable | Description |
|---|---|
| `AWS_REGION` | Bucket region, e.g. `us-east-1` |
| `S3_BUCKET` | Bucket name |
| `PORT` | HTTP port (default 8080) |
| `MAX_UPLOAD_MB` | Upload size cap (default 100) |
| `AWS_PROFILE` | Optional — profile that assumes the scoped uploader role |

`.env` is gitignored — never commit it.

## Credentials

The app uses the standard AWS SDK credential chain. Two ways to run it:

**Quick (your own CLI session):**

> Note: `aws login` root/browser sessions are not readable by the Go SDK
> directly, and root cannot assume IAM roles. Export them first:

```bash
eval "$(aws configure export-credentials --format env)"
go run .
```

**Least-privilege (recommended):** use the dedicated app user that can only
assume the bucket-scoped role. Add to `~/.aws/credentials`:

```ini
[s3uploader-user]
aws_access_key_id     = <APP_USER_ACCESS_KEY_ID>
aws_secret_access_key = <APP_USER_SECRET_ACCESS_KEY>
```

and to `~/.aws/config`:

```ini
[profile s3uploader]
role_arn       = arn:aws:iam::<ACCOUNT_ID>:role/dev-s3-photo-uploader
source_profile = s3uploader-user
```

then set `AWS_PROFILE=s3uploader` in `.env` and run `go run .`.

The role can only `ListBucket`, `PutObject`, `GetObject`, and
`RestoreObject` on the one photo bucket — no deletes, no other buckets.

## Run

```bash
go run .
# listening on http://localhost:8080 (or your PORT)
```

Open the page, drop a photo, pick a storage class, and upload. The page
shows upload progress, then the EXIF table, color swatches, and a map pin
when GPS data is present. Previously uploaded files are listed below;
Glacier Deep Archive objects show an "Archived" badge instead of a download
link (retrieval takes ~12–48 hours via `aws s3api restore-object`).

## Test

```bash
go test ./...
```

The photo package tests use a real GPS-tagged JPEG in
`internal/photo/testdata/` and need no AWS access or network.

## API

| Endpoint | Description |
|---|---|
| `POST /api/upload` | Multipart fields `file` and `storage_class` (`STANDARD` or `DEEP_ARCHIVE`). Returns key, presigned URL, EXIF, and color palette JSON. |
| `GET /api/files` | Lists bucket contents with storage class and retrievability. |

## Infrastructure

The bucket and IAM resources are managed as Terraform/Terragrunt IaC in the
separate `gitops_iac` workspace (module `s3-photo-uploader`, stack
`live/dev/us-east-1/s3-uploader`). It creates:

- The photo bucket (public access blocked, AES256 encryption)
- IAM role `dev-s3-photo-uploader` scoped to that bucket only
- IAM user `dev-s3uploader-app` whose sole permission is assuming that role

Get the app user's key from the stack outputs:

```bash
terragrunt output -raw app_access_key_id
terragrunt output -raw app_secret_access_key
```
