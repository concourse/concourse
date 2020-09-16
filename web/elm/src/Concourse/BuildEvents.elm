module Concourse.BuildEvents exposing
    ( dateFromSeconds
    , decodeBuildEvent
    , decodeBuildEventEnvelope
    , decodeErrorEvent
    , decodeFinishResource
    , decodeOrigin
    )

import Build.StepTree.Models exposing (BuildEvent(..), BuildEventEnvelope, Origin)
import Concourse
import Concourse.BuildStatus
import Dict
import Json.Decode
import Time


decodeBuildEventEnvelope : Json.Decode.Decoder BuildEventEnvelope
decodeBuildEventEnvelope =
    let
        typeDecoder =
            Json.Decode.field
                "type"
                Json.Decode.string

        urlDecoder =
            Json.Decode.at [ "target", "url" ] Json.Decode.string

        dataDecoder =
            typeDecoder
                |> Json.Decode.andThen
                    (\t ->
                        case t of
                            "end" ->
                                Json.Decode.succeed End

                            "open" ->
                                Json.Decode.succeed Opened

                            "error" ->
                                Json.Decode.succeed NetworkError

                            _ ->
                                Json.Decode.field "data" Json.Decode.string
                                    |> Json.Decode.andThen
                                        (\rawEvent ->
                                            case
                                                Json.Decode.decodeString
                                                    decodeBuildEvent
                                                    rawEvent
                                            of
                                                Ok event ->
                                                    Json.Decode.succeed event

                                                Err err ->
                                                    Json.Decode.fail <|
                                                        Json.Decode.errorToString err
                                        )
                    )
    in
    Json.Decode.map2 BuildEventEnvelope
        dataDecoder
        urlDecoder


decodeBuildEvent : Json.Decode.Decoder BuildEvent
decodeBuildEvent =
    Json.Decode.field "event" Json.Decode.string
        |> Json.Decode.andThen
            (\eventType ->
                case eventType of
                    "status" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 BuildStatus
                                (Json.Decode.field "status" Concourse.BuildStatus.decodeBuildStatus)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "log" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map3 Log
                                (Json.Decode.field "origin" <| Json.Decode.lazy (\_ -> decodeOrigin))
                                (Json.Decode.field "payload" Json.Decode.string)
                                (Json.Decode.maybe <| Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "selected-worker" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map3 SelectedWorker
                                (Json.Decode.field "origin" <| Json.Decode.lazy (\_ -> decodeOrigin))
                                (Json.Decode.field "selected_worker" Json.Decode.string)
                                (Json.Decode.maybe <| Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "error" ->
                        Json.Decode.field "data" decodeErrorEvent

                    "initialize-task" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 InitializeTask
                                (Json.Decode.field "origin" <| Json.Decode.lazy (\_ -> decodeOrigin))
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "start-task" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 StartTask
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "finish-task" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map3 FinishTask
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "exit_status" Json.Decode.int)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "initialize" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 Initialize
                                (Json.Decode.field "origin" <| Json.Decode.lazy (\_ -> decodeOrigin))
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "start" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 Start
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "finish" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map3 Finish
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                                (Json.Decode.field "succeeded" Json.Decode.bool)
                            )

                    "initialize-get" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 InitializeGet
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "start-get" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 StartGet
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "finish-get" ->
                        Json.Decode.field "data" (decodeFinishResource FinishGet)

                    "initialize-put" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 InitializePut
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "start-put" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 StartPut
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)
                            )

                    "finish-put" ->
                        Json.Decode.field "data" (decodeFinishResource FinishPut)

                    "set-pipeline-changed" ->
                        Json.Decode.field
                            "data"
                            (Json.Decode.map2 SetPipelineChanged
                                (Json.Decode.field "origin" decodeOrigin)
                                (Json.Decode.field "changed" Json.Decode.bool)
                            )

                    unknown ->
                        Json.Decode.fail ("unknown event type: " ++ unknown)
            )


dateFromSeconds : Int -> Time.Posix
dateFromSeconds =
    Time.millisToPosix << (*) 1000


decodeFinishResource :
    (Origin
     -> Int
     -> Concourse.Version
     -> Concourse.Metadata
     -> Maybe Time.Posix
     -> a
    )
    -> Json.Decode.Decoder a
decodeFinishResource cons =
    Json.Decode.map5 cons
        (Json.Decode.field "origin" decodeOrigin)
        (Json.Decode.field "exit_status" Json.Decode.int)
        (Json.Decode.map
            (Maybe.withDefault Dict.empty)
            << Json.Decode.maybe
         <|
            Json.Decode.field "version" Concourse.decodeVersion
        )
        (Json.Decode.map
            (Maybe.withDefault [])
            << Json.Decode.maybe
         <|
            Json.Decode.field "metadata" Concourse.decodeMetadata
        )
        (Json.Decode.maybe <| Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)


decodeErrorEvent : Json.Decode.Decoder BuildEvent
decodeErrorEvent =
    Json.Decode.map3
        Error
        (Json.Decode.field "origin" decodeOrigin)
        (Json.Decode.field "message" Json.Decode.string)
        (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.int)


decodeOrigin : Json.Decode.Decoder Origin
decodeOrigin =
    Json.Decode.map2 Origin
        (Json.Decode.map (Maybe.withDefault "") << Json.Decode.maybe <| Json.Decode.field "source" Json.Decode.string)
        (Json.Decode.field "id" Json.Decode.string)
