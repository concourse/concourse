module Concourse.Login exposing (..)

import Base64
import Http
import Task exposing (Task)
import Concourse


noAuth : String -> Task Http.Error Concourse.AuthToken
noAuth teamName =
    Http.toTask <| Http.get ("/api/v1/teams/" ++ teamName ++ "/auth/token") Concourse.decodeAuthToken


basicAuth : String -> String -> String -> Task Http.Error Concourse.AuthToken
basicAuth teamName username password =
    Http.toTask <|
        Http.request
            { method = "GET"
            , url = "/api/v1/teams/" ++ teamName ++ "/auth/token"
            , headers = [ encodedAuthHeader username password ]
            , body = Http.emptyBody
            , expect = Http.expectJson Concourse.decodeAuthToken
            , timeout = Nothing
            , withCredentials = False
            }


encodedAuthHeader : String -> String -> Http.Header
encodedAuthHeader username password =
    Http.header "Authorization" <|
        case Base64.encode (username ++ ":" ++ password) of
            Ok code ->
                "Basic " ++ code

            Err err ->
                -- hacky but prevents a lot of type system pain
                "!!! error: " ++ err
