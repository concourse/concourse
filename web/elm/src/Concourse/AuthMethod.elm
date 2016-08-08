module Concourse.AuthMethod exposing (..)

import Http
import Json.Decode exposing ((:=))
import Task exposing (Task)

type AuthMethod
  = BasicMethod
  | OAuthMethod OAuthAuthMethod

type alias OAuthAuthMethod =
  { displayName: String
  , authURL: String
  }

fetchAuthMethods : String -> Task Http.Error (List AuthMethod)
fetchAuthMethods teamName =
  Http.get decodeAuthMethods <| "/api/v1/teams/" ++ teamName ++ "/auth/methods"

decodeAuthMethods : Json.Decode.Decoder (List AuthMethod)
decodeAuthMethods =
  Json.Decode.list <|
    Json.Decode.customDecoder
      ( Json.Decode.object3
          (,,)
          ("type" := Json.Decode.string)
          (Json.Decode.maybe <| "display_name" := Json.Decode.string)
          (Json.Decode.maybe <| "auth_url" := Json.Decode.string)
      )
      authMethodFromTuple

authMethodFromTuple : (String, Maybe String, Maybe String) -> Result String AuthMethod
authMethodFromTuple tuple =
  case tuple of
    ("basic", _, _) -> Ok BasicMethod
    ("oauth", Just displayName, Just authURL) ->
      Ok (OAuthMethod { displayName = displayName, authURL = authURL })
    ("oauth", _, _) -> Err "missing fields in oauth auth method"
    _ -> Err "unknown value for auth method type"
