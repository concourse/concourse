module Concourse.Login exposing (..)

import Base64
import Http
import Json.Decode
import Dict
import Task exposing (Task)
import Concourse


noAuth : String -> Task Http.Error Concourse.AuthToken
noAuth teamName =
    Http.toTask <|
        Http.request
            { method = "GET"
            , url = "/api/v1/teams/" ++ teamName ++ "/auth/token"
            , headers = []
            , body = Http.emptyBody
            , expect = Http.expectStringResponse parseResponse
            , timeout = Nothing
            , withCredentials = False
            }


parseResponse : Http.Response String -> Result String Concourse.AuthSession
parseResponse response =
    let
        authToken =
            Json.Decode.decodeString Concourse.decodeAuthToken response.body
    in
        -- TODO: header can be any case
        flip always (Debug.log ("header") (Dict.get "X-Csrf-Token" response.headers)) <|
            authToken


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
