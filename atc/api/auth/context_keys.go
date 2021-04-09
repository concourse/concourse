package auth

import "github.com/concourse/concourse/atc"

const BuildContextKey atc.ContextKey = "build"
const PipelineContextKey atc.ContextKey = "pipeline"
const LoggerContextKey atc.ContextKey = "logger"

const AuthCookieName = "skymarshal_auth"
const CSRFRequiredKey atc.ContextKey = "CSRFRequired"
const CSRFHeaderName = "X-Csrf-Token"
