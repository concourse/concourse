module Concourse.Login exposing (..)

import Base64
import Http
import Task exposing (Task)

import Concourse

noAuth : String -> Task Http.Error Concourse.AuthToken
noAuth teamName =
  Http.get Concourse.decodeAuthToken ("/api/v1/teams/" ++ teamName ++ "/auth/token")

basicAuth : String -> String -> String -> Task Http.Error Concourse.AuthToken
basicAuth teamName username password =
  case encodedAuthHeader username password of
    Nothing ->
      Task.fail <| Http.UnexpectedPayload "could-not-encode"
    Just header ->
      let
        delivery =
          Http.send Http.defaultSettings
            { verb = "GET"
            , headers = [ header ]
            , url = "/api/v1/teams/" ++ teamName ++ "/auth/token"
            , body = Http.empty
            }
      in
        Http.fromJson Concourse.decodeAuthToken delivery

encodedAuthHeader : String -> String -> Maybe (String, String)
encodedAuthHeader username password =
  case Base64.encode (username ++ ":" ++ password) of
    Ok code ->
      Just
        ( "Authorization"
        , "Basic " ++ code
        )
    Err err ->
      Nothing

handleResponse : Http.Response -> Task Http.Error ()
handleResponse response =
  if 200 <= response.status && response.status < 300 then
    Task.succeed ()
  else
    Task.fail (Http.BadResponse response.status response.statusText)

promoteHttpError : Http.RawError -> Http.Error
promoteHttpError rawError =
  case rawError of
    Http.RawTimeout -> Http.Timeout
    Http.RawNetworkError -> Http.NetworkError
