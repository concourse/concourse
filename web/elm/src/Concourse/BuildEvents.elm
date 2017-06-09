module Concourse.BuildEvents exposing (..)

import Date exposing (Date)
import Dict exposing (Dict)
import Json.Decode
import Concourse
import EventSource


type BuildEvent
    = BuildStatus Concourse.BuildStatus Date
    | Initialize Origin
    | StartTask Origin
    | FinishTask Origin Int
    | FinishGet Origin Int Concourse.Version Concourse.Metadata
    | FinishPut Origin Int Concourse.Version Concourse.Metadata
    | Log Origin String
    | Error Origin String
    | BuildError String


type Msg
    = Opened
    | Errored
    | Event (Result String BuildEvent)
    | End


type alias BuildEventEnvelope =
    { event : String
    , version : String
    , value : Json.Decode.Value
    }


type alias Origin =
    { source : String
    , id : String
    }


subscribe : Int -> Sub Msg
subscribe build =
    EventSource.listen ( "/api/v1/builds/" ++ toString build ++ "/events", [ "end", "event" ] ) parseMsg


parseMsg : EventSource.Msg -> Msg
parseMsg msg =
    case msg of
        EventSource.Event { name, data } ->
            case name of
                Just "end" ->
                    End

                Just "event" ->
                    Event (parseEvent data)

                _ ->
                    Event (Err ("unknown event type: " ++ toString name ++ " (data: " ++ toString data ++ ")"))

        EventSource.Opened ->
            Opened

        EventSource.Errored ->
            Errored


parseEvent : String -> Result String BuildEvent
parseEvent data =
    Json.Decode.decodeString decode data


decode : Json.Decode.Decoder BuildEvent
decode =
    customDecoder decodeEnvelope decodeEvent


decodeEnvelope : Json.Decode.Decoder BuildEventEnvelope
decodeEnvelope =
    Json.Decode.map3 BuildEventEnvelope
        (Json.Decode.field "event" Json.Decode.string)
        (Json.Decode.field "version" Json.Decode.string)
        (Json.Decode.field "data" Json.Decode.value)


dateFromSeconds : Float -> Date
dateFromSeconds =
    Date.fromTime << ((*) 1000)


decodeEvent : BuildEventEnvelope -> Result String BuildEvent
decodeEvent e =
    case e.event of
        "status" ->
            Json.Decode.decodeValue (Json.Decode.map2 BuildStatus (Json.Decode.field "status" Concourse.decodeBuildStatus) (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.float)) e.value

        "log" ->
            Json.Decode.decodeValue (Json.Decode.map2 Log (Json.Decode.field "origin" decodeOrigin) (Json.Decode.field "payload" Json.Decode.string)) e.value

        "error" ->
            Json.Decode.decodeValue decodeErrorEvent e.value

        "initialize" ->
            Json.Decode.decodeValue (Json.Decode.map Initialize (Json.Decode.field "origin" decodeOrigin)) e.value

        "start-task" ->
            Json.Decode.decodeValue (Json.Decode.map StartTask (Json.Decode.field "origin" decodeOrigin)) e.value

        "finish-task" ->
            Json.Decode.decodeValue (Json.Decode.map2 FinishTask (Json.Decode.field "origin" decodeOrigin) (Json.Decode.field "exit_status" Json.Decode.int)) e.value

        "finish-get" ->
            Json.Decode.decodeValue (decodeFinishResource FinishGet) e.value

        "finish-put" ->
            Json.Decode.decodeValue (decodeFinishResource FinishPut) e.value

        unknown ->
            Err ("unknown event type: " ++ unknown)


decodeFinishResource : (Origin -> Int -> Concourse.Version -> Concourse.Metadata -> a) -> Json.Decode.Decoder a
decodeFinishResource cons =
    Json.Decode.map4 cons
        (Json.Decode.field "origin" decodeOrigin)
        (Json.Decode.field "exit_status" Json.Decode.int)
        (Json.Decode.map (Maybe.withDefault Dict.empty) << Json.Decode.maybe <| Json.Decode.field "version" Concourse.decodeVersion)
        (Json.Decode.map (Maybe.withDefault []) << Json.Decode.maybe <| Json.Decode.field "metadata" Concourse.decodeMetadata)


decodeErrorEvent : Json.Decode.Decoder BuildEvent
decodeErrorEvent =
    Json.Decode.oneOf
        [ Json.Decode.map2 Error (Json.Decode.field "origin" decodeOrigin) (Json.Decode.field "message" Json.Decode.string)
        , Json.Decode.map BuildError (Json.Decode.field "message" Json.Decode.string)
        ]


decodeOrigin : Json.Decode.Decoder Origin
decodeOrigin =
    Json.Decode.map2 Origin
        (Json.Decode.map (Maybe.withDefault "") << Json.Decode.maybe <| Json.Decode.field "source" Json.Decode.string)
        (Json.Decode.field "id" Json.Decode.string)


customDecoder : Json.Decode.Decoder b -> (b -> Result String a) -> Json.Decode.Decoder a
customDecoder decoder toResult =
    Json.Decode.andThen
        (\a ->
            case toResult a of
                Ok b ->
                    Json.Decode.succeed b

                Err err ->
                    Json.Decode.fail err
        )
        decoder
