module Concourse.ListAllJobsEvent exposing (JobUpdate(..), ListAllJobsEvent(..), decodeListAllJobsEvent)

import Api.EventSource exposing (decodeData)
import Concourse exposing (decodeJob)
import Json.Decode


type ListAllJobsEvent
    = Initial (List Concourse.Job)
    | Patch (List JobUpdate)


type JobUpdate
    = Put Int Concourse.Job
    | Delete Int


decodeListAllJobsEvent : String -> Json.Decode.Decoder ListAllJobsEvent
decodeListAllJobsEvent eventType =
    case eventType of
        "initial" ->
            decodeData (Json.Decode.list decodeJob |> Json.Decode.map Initial)

        "patch" ->
            decodeData (Json.Decode.list decodeJobUpdate |> Json.Decode.map Patch)

        invalidType ->
            Json.Decode.fail <| "unsupported event type " ++ invalidType


decodeJobUpdate : Json.Decode.Decoder JobUpdate
decodeJobUpdate =
    let
        decodeID =
            Json.Decode.field "id" Json.Decode.int
    in
    Json.Decode.field "eventType" Json.Decode.string
        |> Json.Decode.andThen
            (\t ->
                case t of
                    "PUT" ->
                        Json.Decode.map2 Put
                            decodeID
                            (Json.Decode.field "job" decodeJob)

                    "DELETE" ->
                        Json.Decode.map Delete
                            decodeID

                    invalidType ->
                        Json.Decode.fail <| "invalid update event type " ++ invalidType
            )
