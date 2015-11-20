module EventSource where

{-| This library provides a Task-based interface for attaching to EventSource
endpoints and subscribing to events.
# Creating an EventSource
@docs EventSource, connect
# Subscribing to events
@docs Event, on
# Closing the event source
@docs close
-}

import Time
import Task
import Native.EventSource

{-| An opaque type representing the EventSource.
-}
type EventSource = EventSource

{-| Represent events that have appeared from the EventSource.
-}
type alias Event =
  { lastEventId : Maybe String
  , name : Maybe String
  , data : String
  }

{-| Configure the EventSource with callbacks.
  * `onOpen` corresponds to the EventSource `onopen` callback.
  * `onError` corresponds to the EventSource `onerror` callback.
-}
type alias Settings =
  { onOpen : Maybe (Signal.Address ())
  , onError : Maybe (Signal.Address ())
  }

{-| Connect to an EventSource endpoint. The `Settings` argument allows you to
listen for the connection opening and erroring.
The task produces the EventSource immediately, and never fails. Event handlers
should be registered with `on`.
-}
connect : String -> Settings -> Task.Task x EventSource
connect =
  Native.EventSource.connect

{-| Listen for an event, emitting it to the given address.
The task produces the EventSource to help with chaining:
    connect "http://example.com" `andThen` on "event1" addr1 `andThen` on "event2" addr2
-}
on : String -> Signal.Address Event -> EventSource -> Task.Task x EventSource
on =
  Native.EventSource.on

{-| Close the event source.
-}
close : EventSource -> Task.Task x ()
close =
  Native.EventSource.close
