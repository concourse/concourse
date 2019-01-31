module Concourse.BuildEvents exposing
    ( BuildEvent(..)
    , Msg(..)
    , Origin
    , dateFromSeconds
    , decodeBuildEvent
    , decodeBuildEventEnvelope
    , decodeErrorEvent
    , decodeFinishResource
    , decodeOrigin
    , parseEvent
    , parseEvents
    , parseEventsFromIndex
    , parseMsg
    , subscribe
    )

import Array exposing (Array)
import Concourse
import Date exposing (Date)
import Dict exposing (Dict)
import EventSource
import EventSource.LowLevel as ES
import Json.Decode
import Subscription exposing (Subscription(..))


type BuildEvent
    = BuildStatus Concourse.BuildStatus Date
    | Initialize Origin
    | StartTask Origin
    | FinishTask Origin Int
    | FinishGet Origin Int Concourse.Version Concourse.Metadata
    | FinishPut Origin Int Concourse.Version Concourse.Metadata
    | Log Origin String (Maybe Date)
    | Error Origin String
    | BuildError String
    | End


type Msg
    = Opened
    | Errored
    | Events (Result String (Array.Array BuildEvent))


type alias Origin =
    { source : String
    , id : String
    }


decodeBuildEventEnvelope : Json.Decode.Decoder BuildEvent
decodeBuildEventEnvelope =
    Json.Decode.field "event" Json.Decode.string |> Json.Decode.andThen decodeBuildEvent


decodeBuildEvent : String -> Json.Decode.Decoder BuildEvent
decodeBuildEvent eventType =
    case eventType of
        "status" ->
            Json.Decode.field
                "data"
                (Json.Decode.map2 BuildStatus
                    (Json.Decode.field "status" Concourse.decodeBuildStatus)
                    (Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.float)
                )

        "log" ->
            Json.Decode.field
                "data"
                (Json.Decode.map3 Log
                    (Json.Decode.field "origin" <| Json.Decode.lazy (\_ -> decodeOrigin))
                    (Json.Decode.field "payload" Json.Decode.string)
                    (Json.Decode.maybe <| Json.Decode.field "time" <| Json.Decode.map dateFromSeconds Json.Decode.float)
                )

        "error" ->
            Json.Decode.field "data" decodeErrorEvent

        "initialize" ->
            Json.Decode.field
                "data"
                (Json.Decode.map Initialize (Json.Decode.field "origin" decodeOrigin))

        "initialize-task" ->
            Json.Decode.field
                "data"
                (Json.Decode.map Initialize (Json.Decode.field "origin" decodeOrigin))

        "start-task" ->
            Json.Decode.field
                "data"
                (Json.Decode.map StartTask (Json.Decode.field "origin" decodeOrigin))

        "finish-task" ->
            Json.Decode.field
                "data"
                (Json.Decode.map2 FinishTask
                    (Json.Decode.field "origin" decodeOrigin)
                    (Json.Decode.field "exit_status" Json.Decode.int)
                )

        "finish-get" ->
            Json.Decode.field "data" (decodeFinishResource FinishGet)

        "finish-put" ->
            Json.Decode.field "data" (decodeFinishResource FinishPut)

        unknown ->
            Json.Decode.fail ("unknown event type: " ++ unknown)


subscribe : Int -> Subscription Msg
subscribe build =
    FromEventSource ( "/api/v1/builds/" ++ toString build ++ "/events", [ "end", "event" ] ) parseMsg


parseEvents : Array.Array ES.Event -> Result String (Array.Array BuildEvent)
parseEvents evs =
    -- this is hard to read, but faster than a fold or using a List
    parseEventsFromIndex evs (Array.initialize (Array.length evs) (\_ -> End)) 0


parseEventsFromIndex : Array.Array ES.Event -> Array.Array BuildEvent -> Int -> Result String (Array.Array BuildEvent)
parseEventsFromIndex evs acc i =
    case Array.get i evs of
        Nothing ->
            Ok acc

        Just ev ->
            parseEvent ev
                |> Result.andThen (\ev -> parseEventsFromIndex evs (Array.set i ev acc) (i + 1))


parseMsg : EventSource.Msg -> Msg
parseMsg msg =
    case msg of
        EventSource.Events evs ->
            Events (parseEvents evs)

        EventSource.Opened ->
            Opened

        EventSource.Errored ->
            Errored


parseEvent : ES.Event -> Result String BuildEvent
parseEvent event =
    case ( event.name, event.data ) of
        ( Just "end", data ) ->
            Ok End

        ( Just "event", data ) ->
            Json.Decode.decodeString decodeBuildEventEnvelope data

        ( name, data ) ->
            Err ("unknown event type: " ++ toString name ++ " (data: " ++ toString data ++ ")")


dateFromSeconds : Float -> Date
dateFromSeconds =
    Date.fromTime << (*) 1000


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
