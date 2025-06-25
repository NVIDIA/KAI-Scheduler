{{/* 
Copyright 2025 NVIDIA CORPORATION
SPDX-License-Identifier: Apache-2.0
*/}}

{{/*
Template for a webhook job
*/}}
{{- define "kai-scheduler.webhookJob" -}}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .jobName }}
  labels:
    app: {{ .jobName }}
  annotations:
    helm.sh/hook: post-install,post-upgrade
    helm.sh/hook-weight: "{{ .weight | default "0" }}"
    helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded
spec:
  template:
    spec:
      serviceAccountName: webhookmanager
      restartPolicy: Never
      containers:
        - name: {{ .jobName }}
          image: "{{ .Values.global.registry }}/{{ .Values.webhookmanager.image.name }}:{{ .Chart.Version }}"
          imagePullPolicy: {{ .Values.webhookmanager.image.pullPolicy }}
          {{- if .Values.webhookmanager.resources }}
          resources:
            {{- toYaml .Values.webhookmanager.resources | nindent 12 }}
          {{- end }}
          {{- if .Values.global.securityContext }}
          securityContext:
            {{- toYaml .Values.global.securityContext | nindent 12 }}
          {{- end }}
          args:
            - --namespace={{ .Release.Namespace }}
            - --patch-webhooks
            - --secret-name={{ .secretName }}
            {{- if .validatingWebhookName }}
            - --validating-webhook-name={{ .validatingWebhookName }}
            {{- end }}
            {{- if .mutatingWebhookName }}
            - --mutating-webhook-name={{ .mutatingWebhookName }}
            {{- end }}
      {{- if .Values.global.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml .Values.global.imagePullSecrets | nindent 8 }}
      {{- end }}
      {{- if .Values.global.nodeSelector }}
      nodeSelector:
        {{- toYaml .Values.global.nodeSelector | nindent 8 }}
      {{- end }}
      {{- if .Values.global.affinity }}
      affinity:
        {{- toYaml .Values.global.affinity | nindent 8 }}
      {{- end }}
      {{- if .Values.global.tolerations }}
      tolerations:
        {{- toYaml .Values.global.tolerations | nindent 8 }}
      {{- end }}
{{- end -}}

{{/*
Template for a pre-install webhook job
*/}}
{{- define "kai-scheduler.preInstallWebhookJob" -}}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .jobName }}
  labels:
    app: {{ .jobName }}
  annotations:
    helm.sh/hook: pre-install,pre-upgrade
    helm.sh/hook-weight: "{{ .weight | default "0" }}"
    helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded
spec:
  template:
    spec:
      serviceAccountName: webhookmanager
      restartPolicy: Never
      containers:
        - name: {{ .jobName }}
          image: "{{ .Values.global.registry }}/{{ .Values.webhookmanager.image.name }}:{{ .Chart.Version }}"
          imagePullPolicy: {{ .Values.webhookmanager.image.pullPolicy }}
          {{- if .Values.webhookmanager.resources }}
          resources:
            {{- toYaml .Values.webhookmanager.resources | nindent 12 }}
          {{- end }}
          {{- if .Values.global.securityContext }}
          securityContext:
            {{- toYaml .Values.global.securityContext | nindent 12 }}
          {{- end }}
          args:
            - --namespace={{ .Release.Namespace }}
            - --service-name={{ .serviceName }}
            - --upsert-secret
            - --secret-name={{ .secretName }}
      {{- if .Values.global.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml .Values.global.imagePullSecrets | nindent 8 }}
      {{- end }}
      {{- if .Values.global.nodeSelector }}
      nodeSelector:
        {{- toYaml .Values.global.nodeSelector | nindent 8 }}
      {{- end }}
      {{- if .Values.global.affinity }}
      affinity:
        {{- toYaml .Values.global.affinity | nindent 8 }}
      {{- end }}
      {{- if .Values.global.tolerations }}
      tolerations:
        {{- toYaml .Values.global.tolerations | nindent 8 }}
      {{- end }}
{{- end -}}
{{/*
Renders a map into a comma separated list of key=value pairs
*/}}
{{- define "render.map" -}}
{{- $items := list }}{{/* initializes an empty list */}}
{{- range $k, $v := . }}
  {{- $items = append $items (printf "%s=%s" $k $v) }}
{{- end }}
{{- join "," $items }}
{{- end }}