#!/usr/bin/env bash

set -exuo pipefail

ENV=${ENV:-prod}
ECR_REPO=${ECR_REPO:-519856050701.dkr.ecr.us-west-2.amazonaws.com}
DRY_RUN=${DRY_RUN:-}

repo_name=boring-registry
chart_name=boring-registry
internal_version=$(cat INTERNAL_VERSION)
chart_version=$(grep "^version:" "./helm/${chart_name}/Chart.yaml" | cut -d' ' -f 2)

# Update Chart.yaml version if it doesn't match INTERNAL_VERSION
if [ "${chart_version}" != "${internal_version}" ]; then
  echo "Updating Chart.yaml version from ${chart_version} to ${internal_version}"
  sed -i.bak "s/^version: .*/version: ${internal_version}/" "./helm/${chart_name}/Chart.yaml"
  chart_version="${internal_version}"
fi

downloaded_chart_name="${chart_name}-${chart_version}.tgz"

# Ensure cleanup on exit
trap "rm -f ${downloaded_chart_name}" EXIT

if [ -f "./helm/${chart_name}/Chart.yaml" ]; then
  packaged_chart=$(helm package "./helm/${chart_name}" | grep "Successfully packaged chart" | sed 's/.*: //')
else
  echo "chart ${chart_name} not found in ${repo_name}"
  exit 1
fi

real_chart_name=$(helm show chart "${packaged_chart}" | grep "^name: " | cut -d' ' -f 2)
if [ "${real_chart_name}" != "${chart_name}" ]; then
  echo "Unexpected chart name '${real_chart_name}' (was expecting '${chart_name}')"
  exit 1
fi

oci_path="oci://${ECR_REPO}/helm/${ENV}/confluentinc/${repo_name}"

if helm show chart "${oci_path}/${chart_name}" --version "${chart_version}" 1>/dev/null 2>&1; then
  echo "${chart_name}:${chart_version} already exists"
  exit 1
fi

if [ -z "${DRY_RUN}" ]; then
  echo "Pushing ${packaged_chart} to ${oci_path}"
  helm push "${packaged_chart}" "${oci_path}"
else
  echo "Would push ${packaged_chart} to ${oci_path}"
fi
