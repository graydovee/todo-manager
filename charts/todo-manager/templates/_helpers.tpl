{{/*
Expand the name of the chart.
*/}}
{{- define "todo-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "todo-manager.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "todo-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "todo-manager.labels" -}}
helm.sh/chart: {{ include "todo-manager.chart" . }}
{{ include "todo-manager.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "todo-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "todo-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "todo-manager.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "todo-manager.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Secret name
*/}}
{{- define "todo-manager.secretName" -}}
{{- include "todo-manager.fullname" . }}
{{- end }}

{{/*
ConfigMap name
*/}}
{{- define "todo-manager.configmapName" -}}
{{- include "todo-manager.fullname" . }}
{{- end }}

{{/*
Bundled PostgreSQL fullname (matches bitnami subchart naming)
*/}}
{{- define "todo-manager.postgresql.fullname" -}}
{{- printf "%s-postgresql" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Construct the database DSN based on mode.
For sqlite: returns the file path.
For bundled: constructs a postgres DSN using the subchart service.
For external: constructs a DSN from the external config.
*/}}
{{- define "todo-manager.databaseDSN" -}}
{{- if eq .Values.database.mode "sqlite" }}
{{- .Values.database.sqlite.path }}
{{- else if eq .Values.database.mode "bundled" }}
{{- printf "host=%s port=5432 user=%s dbname=%s sslmode=disable" (include "todo-manager.postgresql.fullname" .) .Values.database.bundled.username .Values.database.bundled.database }}
{{- else if eq .Values.database.mode "external" }}
{{- if eq .Values.database.external.type "postgres" }}
{{- printf "host=%s port=%d user=%s dbname=%s sslmode=disable" .Values.database.external.host (int .Values.database.external.port) .Values.database.external.username .Values.database.external.database }}
{{- else if eq .Values.database.external.type "mysql" }}
{{- printf "%s:PLACEHOLDER@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local" .Values.database.external.username .Values.database.external.host (int .Values.database.external.port) .Values.database.external.database }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Construct the database DSN with actual password (for Secret).
*/}}
{{- define "todo-manager.databaseDSNWithPassword" -}}
{{- if eq .Values.database.mode "bundled" }}
{{- printf "host=%s port=5432 user=%s password=%s dbname=%s sslmode=disable" (include "todo-manager.postgresql.fullname" .) .Values.database.bundled.username .Values.database.bundled.password .Values.database.bundled.database }}
{{- else if eq .Values.database.mode "external" }}
{{- if eq .Values.database.external.type "postgres" }}
{{- printf "host=%s port=%d user=%s password=%s dbname=%s sslmode=disable" .Values.database.external.host (int .Values.database.external.port) .Values.database.external.username .Values.database.external.password .Values.database.external.database }}
{{- else if eq .Values.database.external.type "mysql" }}
{{- printf "%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local" .Values.database.external.username .Values.database.external.password .Values.database.external.host (int .Values.database.external.port) .Values.database.external.database }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Database driver name
*/}}
{{- define "todo-manager.databaseDriver" -}}
{{- if eq .Values.database.mode "sqlite" }}
{{- "sqlite" }}
{{- else if eq .Values.database.mode "bundled" }}
{{- "postgres" }}
{{- else if eq .Values.database.mode "external" }}
{{- .Values.database.external.type }}
{{- end }}
{{- end }}
