module Concourse.BuildEvents exposing (..)

import Date exposing (Date)
import Dict exposing (Dict)
import Json.Decode exposing ((:=))

import Concourse.BuildStatus exposing (BuildStatus)
import Concourse.Metadata exposing (Metadata)
import Concourse.Version exposing (Version)

import EventSource

type BuildEvent
  = BuildStatus BuildStatus Date
  | InitializeTask Origin
  | StartTask Origin
  | FinishTask Origin Int
  | InitializeGet Origin
  | FinishGet Origin Int Version Metadata
  | InitializePut Origin
  | FinishPut Origin Int Version Metadata
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
  EventSource.listen ("/api/v1/builds/" ++ toString build ++ "/events", ["end", "event"]) parseMsg

parseMsg : EventSource.Msg -> Msg
parseMsg msg =
  case msg of
    EventSource.Event {name, data} ->
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
  Json.Decode.customDecoder decodeEnvelope decodeEvent

decodeEnvelope : Json.Decode.Decoder BuildEventEnvelope
decodeEnvelope =
  Json.Decode.object3 BuildEventEnvelope
    ("event" := Json.Decode.string)
    ("version" := Json.Decode.string)
    ("data" := Json.Decode.value)

dateFromSeconds : Float -> Date
dateFromSeconds = Date.fromTime << ((*) 1000)

decodeEvent : BuildEventEnvelope -> Result String BuildEvent
decodeEvent e =
  case e.event of
    "status" ->
      Json.Decode.decodeValue (Json.Decode.object2 BuildStatus ("status" := Concourse.BuildStatus.decode) ("time" := Json.Decode.map dateFromSeconds Json.Decode.float)) e.value

    "log" ->
      Json.Decode.decodeValue (Json.Decode.object2 Log ("origin" := decodeOrigin) ("payload" := Json.Decode.string)) e.value

    "error" ->
      Json.Decode.decodeValue decodeErrorEvent e.value

    "initialize-task" ->
      Json.Decode.decodeValue (Json.Decode.object1 InitializeTask ("origin" := decodeOrigin)) e.value

    "start-task" ->
      Json.Decode.decodeValue (Json.Decode.object1 StartTask ("origin" := decodeOrigin)) e.value

    "finish-task" ->
      Json.Decode.decodeValue (Json.Decode.object2 FinishTask ("origin" := decodeOrigin) ("exit_status" := Json.Decode.int)) e.value

    "initialize-get" ->
      Json.Decode.decodeValue (Json.Decode.object1 InitializeGet ("origin" := decodeOrigin)) e.value

    "finish-get" ->
      Json.Decode.decodeValue (decodeFinishResource FinishGet) e.value

    "initialize-put" ->
      Json.Decode.decodeValue (Json.Decode.object1 InitializePut ("origin" := decodeOrigin)) e.value

    "finish-put" ->
      Json.Decode.decodeValue (decodeFinishResource FinishPut) e.value

    unknown ->
      Err ("unknown event type: " ++ unknown)

decodeFinishResource : (Origin -> Int -> Dict String String -> Metadata -> a) -> Json.Decode.Decoder a
decodeFinishResource cons =
  Json.Decode.object4 cons
    ("origin" := decodeOrigin)
    ("exit_status" := Json.Decode.int)
    (Json.Decode.map (Maybe.withDefault Dict.empty) << Json.Decode.maybe <| "version" := Concourse.Version.decode)
    (Json.Decode.map (Maybe.withDefault []) << Json.Decode.maybe <| "metadata" := Concourse.Metadata.decode)

decodeErrorEvent : Json.Decode.Decoder BuildEvent
decodeErrorEvent =
  Json.Decode.oneOf
    [ Json.Decode.object2 Error ("origin" := decodeOrigin) ("message" := Json.Decode.string)
    , Json.Decode.object1 BuildError ("message" := Json.Decode.string)
    ]

decodeOrigin : Json.Decode.Decoder Origin
decodeOrigin =
  Json.Decode.object2 Origin
    (Json.Decode.map (Maybe.withDefault "") << Json.Decode.maybe <| "source" := Json.Decode.string)
    ("id" := Json.Decode.string)
