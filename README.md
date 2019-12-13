[![Build Status](https://api.travis-ci.com/danitso/clouddk-csi-driver.svg?branch=master)](https://travis-ci.com/danitso/clouddk-csi-driver)

# Kubernetes CSI Driver for Cloud.dk

`clouddk-csi-driver` is a Kubernetes Container Storage Interface ([CSI](https://kubernetes-csi.github.io/docs/)) Driver for [Cloud.dk](https://cloud.dk).

> **WARNING:** This project is under active development and should be considered alpha.

## Introduction

The Container Storage Interface (CSI) is a standard for exposing arbitrary block and file storage storage systems to containerized workloads on Container Orchestration Systems (COs) like Kubernetes. Using CSI third-party storage providers can write and deploy plugins exposing new storage systems in Kubernetes without ever having to touch the core Kubernetes code.

## Preparation

To use CSI drivers, your Kubernetes cluster must allow privileged pods (i.e. --allow-privileged flag must be set to true for both the API server and the kubelet). This is the default for clusters created with `kubeadm`.

## Installation

Follow these simple steps in order to install the driver:

1. Ensure that `kubectl` is configured to reach the cluster

1. Retrieve the API key from [https://my.cloud.dk/account/api-key](https://my.cloud.dk/account/api-key) and encode it

    ```bash
    echo "CLOUDDK_API_KEY: '$(echo "the API key here" | base64 | tr -d '\n')'"
    ```

1. Specify the hardware requirements for the network storage servers

    ```bash
    echo "CLOUDDK_SERVER_MEMORY: '$(echo 4096 | base64 | tr -d '\n')'" \
    && echo "CLOUDDK_SERVER_PROCESSORS: '$(echo 2 | base64 | tr -d '\n')'"
    ```

1. Create a new SSH key pair

    ```bash
    rm -f /tmp/clouddk_ssh_key* \
        && ssh-keygen -b 4096 -t rsa -f /tmp/clouddk_ssh_key -q -N "" \
        && echo "CLOUDDK_SSH_PRIVATE_KEY: '$(cat /tmp/clouddk_ssh_key | base64 | tr -d '\n' | base64 | tr -d '\n')'" \
        && echo "CLOUDDK_SSH_PUBLIC_KEY: '$(cat /tmp/clouddk_ssh_key.pub | base64 | tr -d '\n' | base64 | tr -d '\n')'"
    ```

1. Create a new file called `config.yaml` with the following contents:

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: clouddk-csi-driver-config
      namespace: kube-system
    type: Opaque
    data:
      CLOUDDK_API_ENDPOINT: 'aHR0cHM6Ly9hcGkuY2xvdWQuZGsvdjEK'
      CLOUDDK_API_KEY: 'The encoded API key generated in step 2'
      CLOUDDK_SERVER_MEMORY: 'The encoded value generated in step 3'
      CLOUDDK_SERVER_PROCESSORS: 'The encoded value generated in step 3'
      CLOUDDK_SSH_PRIVATE_KEY: 'The encoded private SSH key generated in step 4'
      CLOUDDK_SSH_PUBLIC_KEY: 'The encoded public SSH key generated in step 4'
    ```

1. Create the secret in `config.yaml` using `kubectl`

    ```bash
    kubectl apply -f ./config.yaml
    ```

1. Deploy the driver and the sidecars using `kubectl`

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/danitso/clouddk-csi-driver/master/deployment.yaml
    ```

1. Verify that `clouddk-csi-controller` and `clouddk-csi-node` pods are being created and wait for them to reach a `Running` state

    ```bash
    kubectl get pods -l k8s-app=clouddk-csi-controllers -n kube-system
    kubectl get pods -l k8s-app=clouddk-csi-nodes -n kube-system
    ```

## Features

### PersistentVolume

The `clouddk-csi-driver` plugin adds support for Persistent Volumes based on NFS. The volumes must be created with the `ReadWriteMany` capability.
