# Kubernetes CSI Driver for Cloud.dk

`clouddk-csi-driver` is a Kubernetes Container Storage Interface ([CSI](https://kubernetes-csi.github.io/docs/)) Driver for [Cloud.dk](https://cloud.dk).

> **WARNING:** This project is under active development and should be considered alpha.

## Introduction

The Container Storage Interface (CSI) is a standard for exposing arbitrary block and file storage storage systems to containerized workloads on Container Orchestration Systems (COs) like Kubernetes. Using CSI third-party storage providers can write and deploy plugins exposing new storage systems in Kubernetes without ever having to touch the core Kubernetes code.

## Preparation

Work in progress.

## Installation

Work in progress.

## Features

### PersistentVolume

The `clouddk-csi-driver` plugin adds support for Persistent Volumes based on NFS.

**NOTE**: Support is currently limited to `ReadWriteMany` volumes due to lack of support for moving disks from one server to another. We expect this feature to become available in the Cloud.dk API sometime in the future.
