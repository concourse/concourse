module EventSource.LowLevel exposing (..)

{-| This library provides a Task-based interface for attaching to EventSource
endpoints and subscribing to events.


# Creating an EventSource

@docs EventSource, open


# Subscribing to events

@docs Event, on


# Closing the event source

@docs close

-}

import Task exposing (Task)
import Native.EventSource


{-| An opaque type representing the EventSource.
-}
type EventSource
    = EventSource


{-| Represent events that have appeared from the EventSource.
-}
type alias Event =
    { lastEventId : Maybe String
    , name : Maybe String
    , data : String
    }


{-| Configure the EventSource with callbacks.

  - `onOpen` corresponds to the EventSource `onopen` callback.
  - `onError` corresponds to the EventSource `onerror` callback.

-}
type alias Settings =
    { events : List String
    , onEvent : Event -> Task Never ()
    , onOpen : EventSource -> Task Never ()
    , onError : () -> Task Never ()
    }


{-| Connect to an EventSource endpoint. The `Settings` argument allows you to
listen for the connection opening and erroring.
The task produces the EventSource immediately, and never fails. Event handlers
should be registered with `on`.
-}
open : String -> Settings -> Task.Task x EventSource
open =
    Native.EventSource.open


{-| Close the event source.
-}
close : EventSource -> Task.Task x ()
close =
    Native.EventSource.close
