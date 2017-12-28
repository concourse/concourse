package auth

const BuildContextKey = "build"
const PipelineContextKey = "pipeline"

const TokenTypeBearer = "Bearer"
const teamNameClaimKey = "teamName"
const isAdminClaimKey = "isAdmin"
const csrfTokenClaimKey = "csrf"

const AuthCookieName = "ATC-Authorization"
const CSRFRequiredKey = "CSRFRequired"
const CSRFHeaderName = "X-Csrf-Token"

const authenticated = "authenticated"
const teamNameKey = "teamName"
const isAdminKey = "isAdmin"
const isSystemKey = "system"
const CSRFTokenKey = "csrfToken"
