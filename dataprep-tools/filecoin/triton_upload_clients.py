import requests
import boto3

from botocore.exceptions import ClientError
from botocore.config import Config

import logging
import os

"""
S3 storage client, uses S3 API for uploads
"""
class S3Client:
    def __init__(self, endpoint, bucket, url_format, key_id, application_key, service_name='s3', region=''):
        self.region = region # not used now but would be needed for some s3
        self.endpoint = endpoint # environ.get('S3_ENDPOINT')
        self.key_id = key_id #  environ.get('S3_KEY_ID')
        self.bucket = bucket # bucket = environ.get('BUCKET')
        self.application_key = application_key # environ.get('S3_APPLICATION_KEY')
        self.url_format = url_format

    def connect(self):
        self.client = boto3.client(service_name=self.service_name,
                                 endpoint_url=self.endpoint,                # Backblaze endpoint
                                 aws_access_key_id=self.key_id,              # Backblaze keyID
                                 aws_secret_access_key=self.application_key) # Backblaze applicationKey
        return self.client

    def download_file(self, key_name, local_file):
        try:
            self.client.download_file(self.bucket, key_name, local_file)
        except ClientError as ce:
            logging.exception(ce)
            raise

    def read_object(self, key_name):
        try:
            obj = self.client.get_object(Bucket=self.bucket, Key=key_name)
            body = obj['Body'].read()
            return body.decode()
        except ClientError as ce:
            logging.exception(ce)
            raise

    def upload_obj(self, file_obj, object_name):
        raise NotImplementedError

    def upload_file(self, file_name, object_name):
        """Upload a file to an S3 bucket

        :param file_name: File to upload
        :param bucket: Bucket to upload to
        :param object_name: S3 object name. If not specified then file_name is used
        :return: True if file was uploaded, else False
        """

        # If S3 object_name was not specified, use file_name
        if object_name is None:
            object_name = os.path.basename(file_name)

        # Upload the file
        try:
            response = self.client.upload_file(file_name, self.bucket, object_name)
        except ClientError as e:
            logging.exception(e)
            return False
        return True

    def get_directory(self, prefix):
        raise NotImplementedError

    def delete_file(self, object_name):
        try:
            response = self.client.delete_object(Bucket=self.bucket, Key=object_name)
        except ClientError as e:
            logging.exception(e)
            return False
        return True

    def check_exists(self, key_name):
        try:
            obj = self.client.head_object(Bucket=self.bucket, Key=key_name)
        except ClientError as ce:
            if ce.response['Error']['Code'] == "404":
                return (False, -1)
            else:
                logging.exception(ce)
                raise
        else:
            length = obj['ContentLength']
            return (True, length)

    def get_public_url(self, key_name):
        url = self.url_format.format(region=self.region, storage_name=self.bucket, key_name=key_name)
        return url

    def check_public_url(self, public_uri):
        x = requests.head(public_uri)
        if x.status_code == 200:
            return (True, int(x.headers['Content-Length']))
        else:
            return (False, -1)

"""
Client for Bunny CDN

Uses HTTP uploads and downloads.
"""
class BunnyCDNClient:
    def __init__(self, endpoint, storage_name, url_format, key_id, application_key):
        # key_id not used for Bunny but for other similar uploads it could be http user name
        self.endpoint = endpoint
        self.storage_name = storage_name
        self.application_key = application_key
        self.url_format = url_format

    def connect(self):
        return

    def download_file(self, key_name, local_file):
        try:
            url = self.get_public_url(key_name)
            response = requests.get(url)
            response.raise_for_status()
            with open(local_file, 'w') as file:
                file.write(response.text)
        except Exception as ce:
            logging.exception(ce)
            raise

    # download and return content of file
    def read_object(self, key_name):
        try:
            url = self.get_public_url(key_name)
            response = requests.get(url)
            response.raise_for_status()
            return response.text
        except Exception as ce:
            logging.exception(ce)
            raise

    def upload_file(self, file_name, object_name):
        with open(file_name, "rb") as file:
            return self.upload_obj(file, object_name)

    def upload_obj(self, file_obj, object_name):
        """Upload a file

        :param file_name: Source file to upload
        :param object_name: File name to save on remote storage
        :return: True if file was uploaded, else False
        """

        # Upload the file
        try:
            url = "https://%s/%s/%s" % (self.endpoint, self.storage_name, object_name)
            headers = {
                "content-type": "application/octet-stream",
                "AccessKey": self.application_key
            }
            file_data = file_obj.read()
            response = requests.put(url, headers=headers, data=file_data)
            response.raise_for_status()
        except Exception as ce:
            logging.exception(ce)
            return False
        return True

    def get_directory(self, prefix):
        """
        Fetches a directory listning
        """
        try:
            url = "https://%s/%s/%s" % (self.endpoint, self.storage_name, prefix)
            headers = {
                "Accept":   "application/json",
                "AccessKey": self.application_key
            }
            response = requests.get(url, headers=headers)
            response.raise_for_status()
        except Exception as ce:
            logging.exception(ce)
            raise
        return response.json()

    def delete_file(self, object_name):
        try:
            url = "https://%s/%s/%s" % (self.endpoint, self.storage_name, object_name)
            headers = {
                "Accept":   "application/json",
                "AccessKey": self.application_key
            }
            response = requests.delete(url, headers=headers)
            response.raise_for_status()
        except Exception as ce:
            logging.exception(ce)
            raise
        return True

    def check_exists(self, key_name):
        try:
            url = self.get_public_url(key_name)
            response = requests.head(url)
        except Exception as ce:
            logging.exception(ce)
            raise

        if response.status_code == 404:
            return (False, -1)
        if response.status_code > 204:
            raise Exception("Failed to check file exists: %s" % response.status_code)

        length = response.headers['Content-Length']
        return (True, int(length))

    def get_public_url(self, key_name):
        url = self.url_format.format(storage_name=self.storage_name, key_name=key_name)
        return url

    def check_public_url(self, public_uri):
        x = requests.head(public_uri)
        if x.status_code == 200:
            return (True, int(x.headers['Content-Length']))
        return (False, -1)

