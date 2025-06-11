{{- define "enterprise-node.name" -}}
enterprise-node
{{- end -}}

{{- define "enterprise-node.fullname" -}}
{{ .Release.Name }}-enterprise-node
{{- end -}}
