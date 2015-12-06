module Concourse.BuildEvents where

import Date exposing (Date)
import Dict exposing (Dict)
import Json.Decode exposing ((:=))
import Task exposing (Task)

import Concourse.BuildStatus exposing (BuildStatus)
import EventSource exposing (EventSource)

type BuildEvent
  = BuildStatus BuildStatus Date
  | FinishGet Origin Int Version (List MetadataField)
  | FinishPut Origin Int Version (List MetadataField)
  | InitializeTask Origin
  | StartTask Origin
  | FinishTask Origin Int
  | Log Origin String
  | Error Origin String
  | BuildError String

type Action
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

type StepType
  = StepTypeTask
  | StepTypeGet
  | StepTypePut

type alias Version =
  Dict String String

type alias Metadata =
  List MetadataField

type alias MetadataField =
  { name : String
  , value : String
  }

subscribe : Int -> Signal.Address Action -> Task x EventSource
subscribe build actions =
  let
    settings =
      EventSource.Settings
        (Just <| Signal.forwardTo actions (always Opened))
        (Just <| Signal.forwardTo actions (always Errored))

    connect =
      EventSource.connect ("/api/v1/builds/" ++ toString build ++ "/events") settings

    eventsSub =
      EventSource.on "event" <|
        Signal.forwardTo actions (Event << parseEvent)

    endSub =
      EventSource.on "end" <|
        Signal.forwardTo actions (always End)
  in
    connect `Task.andThen` eventsSub `Task.andThen` endSub

parseEvent : EventSource.Event -> Result String BuildEvent
parseEvent e =
  Json.Decode.decodeString decode e.data

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

    "finish-get" ->
      Json.Decode.decodeValue (decodeFinishResource FinishGet) e.value

    "finish-put" ->
      Json.Decode.decodeValue (decodeFinishResource FinishPut) e.value

    unknown ->
      Err ("unknown event type: " ++ unknown)

decodeFinishResource : (Origin -> Int -> Dict String String -> List MetadataField -> a) -> Json.Decode.Decoder a
decodeFinishResource cons =
  Json.Decode.object4 cons
    ("origin" := decodeOrigin)
    ("exit_status" := Json.Decode.int)
    ("version" := decodeVersion)
    (Json.Decode.map (Maybe.withDefault []) << Json.Decode.maybe <| "metadata" := decodeMetadata)

decodeVersion : Json.Decode.Decoder (Dict String String)
decodeVersion =
  Json.Decode.dict Json.Decode.string

decodeMetadata : Json.Decode.Decoder (List MetadataField)
decodeMetadata =
  Json.Decode.list decodeMetadataField

decodeMetadataField : Json.Decode.Decoder MetadataField
decodeMetadataField =
  Json.Decode.object2 MetadataField
    ("name" := Json.Decode.string)
    ("value" := Json.Decode.string)

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

decodeStepType : Json.Decode.Decoder StepType
decodeStepType =
  Json.Decode.customDecoder ("type" := Json.Decode.string) <| \t ->
    case t of
      "task" -> Ok StepTypeTask
      "get" -> Ok StepTypeGet
      "put" -> Ok StepTypePut
      unknown -> Err ("unknown step type: " ++ unknown)
