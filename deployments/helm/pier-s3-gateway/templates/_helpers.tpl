{{/*
Expand the name of the chart.
*/}}
{{- define "pier-s3-gateway.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified app name.
*/}}
{{- define "pier-s3-gateway.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Chart name and version, for the chart label.
*/}}
{{- define "pier-s3-gateway.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "pier-s3-gateway.labels" -}}
helm.sh/chart: {{ include "pier-s3-gateway.chart" . }}
{{ include "pier-s3-gateway.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels. Includes the legacy `app:` label so existing Services /
NetworkPolicies / PDBs that select on it keep matching.
*/}}
{{- define "pier-s3-gateway.selectorLabels" -}}
app: {{ include "pier-s3-gateway.fullname" . }}
app.kubernetes.io/name: {{ include "pier-s3-gateway.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Name of the Secret holding the runtime env (external, generated, or pre-existing).
*/}}
{{- define "pier-s3-gateway.secretName" -}}
{{- if and (not .Values.externalSecret.enabled) .Values.secret.existingSecret -}}
{{- .Values.secret.existingSecret -}}
{{- else -}}
{{- printf "%s-secrets" (include "pier-s3-gateway.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Resolved image reference (tag defaults to the chart appVersion).
*/}}
{{- define "pier-s3-gateway.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}

{{/*
Render NetworkPolicy egress peers ("to:" list) for a target. Pass a dict with a
"target" key holding { namespaceLabels, podLabels, ipBlock }. Preference:
ipBlock > namespace/pod selectors > fall back to namespaceSelector: {} (any
namespace) - the broad fallback is intentionally last so a configured target
always wins, and it emits a comment so an unconfigured policy is visible in the
rendered manifest.
*/}}
{{- define "pier-s3-gateway.egressPeers" -}}
{{- $t := .target -}}
{{- if $t.ipBlock }}
- ipBlock:
    cidr: {{ $t.ipBlock }}
{{- else if or $t.namespaceLabels $t.podLabels }}
- {{ if $t.namespaceLabels -}}
  namespaceSelector:
    matchLabels:
      {{- toYaml $t.namespaceLabels | nindent 6 }}
  {{- end }}
  {{- if $t.podLabels }}
  podSelector:
    matchLabels:
      {{- toYaml $t.podLabels | nindent 6 }}
  {{- end }}
{{- else }}
# WARNING: egress target unscoped (any namespace). Set namespaceLabels/podLabels
# or ipBlock in values to restrict the blast radius.
- namespaceSelector: {}
{{- end }}
{{- end -}}
