#!/usr/bin/env bash

set -euo pipefail

ENV=${ENV:-prod}
ECR_REPO=${ECT_REPO:-519856050701.dkr.ecr.us-west-2.amazonaws.com}
DRY_RUN=${DRY_RUN:-}

repo_name=boring-registry
chart_name=boring-registry
chart_version=$(cat INTERNAL_VERSION)

# Create a temporary directory
temp_dir=$(mktemp -d)

# Ensure cleanup on exit
trap "rm -rf ${temp_dir}" EXIT

if [ -f "${temp_dir}/helm/${chart_name}/Chart.yaml" ]; then
  helm package "${temp_dir}/charts/${chart_name}"
else
  echo "chart ${chart_name} not found in ${repo_name}"
  exit 1
fi
downloaded_chart_name="${chart_name}-${chart_version}.tgz"

real_chart_name=$(helm show chart "${downloaded_chart_name}" | grep "^name: " | cut -d' ' -f 2)
if [ "${real_chart_name}" != "${chart_name}" ]; then
  echo "Unexpected chart name '${real_chart_name}' (was expecting '${chart_name}')"
  exit 1
fi

oci_path="oci://${ECR_REPO}/helm/${ENV}/confluentinc/${repo_name}"

if helm show chart "${oci_path}/${chart_name}" --version "${chart_version}" 1>/dev/null 2>&1; then
  echo "${chart_name}:${chart_version} already exists"
  return
fi

if [ -z "${DRY_RUN}" ]; then
  echo "Pushing ${downloaded_chart_name} to ${oci_path}"
  helm push "${downloaded_chart_name}" "${oci_path}"
else
  echo "Would push ${downloaded_chart_name} to ${oci_path}"
fi
