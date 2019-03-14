module BuildEventsTests exposing (all)

import Array
import Build.Models
import Build.Msgs
import Build.Output
import EventSource.EventSource as EventSource
import Expect
import Json.Encode
import Test exposing (Test, test)


eventData : String
eventData =
    Json.Encode.encode 0 <|
        Json.Encode.object
            [ ( "data"
              , Json.Encode.object
                    [ ( "origin"
                      , Json.Encode.object
                            [ ( "source", Json.Encode.string "stdout" )
                            , ( "id", Json.Encode.string "stepid" )
                            ]
                      )
                    , ( "payload", Json.Encode.string "log message" )
                    ]
              )
            , ( "event", Json.Encode.string "log" )
            ]


all : Test
all =
    test "parseMsg can handle many events, i.e. should use tail recursion" <|
        \_ ->
            Build.Output.parseMsg
                (EventSource.Events <|
                    Array.fromList <|
                        List.repeat 3000
                            { lastEventId = Nothing
                            , name = Just "event"
                            , data = eventData
                            }
                )
                |> Expect.equal
                    (Build.Msgs.Events <|
                        Ok <|
                            Array.fromList <|
                                List.repeat
                                    3000
                                <|
                                    Build.Models.Log
                                        { source = "stdout", id = "stepid" }
                                        "log message"
                                        Nothing
                    )
