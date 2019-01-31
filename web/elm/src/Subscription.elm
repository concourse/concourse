port module Subscription exposing (Subscription(..), map, runSubscription)

import AnimationFrame
import EventSource
import Keyboard
import Mouse
import Msgs exposing (Msg(..))
import Scroll
import Time
import Window


port newUrl : (String -> msg) -> Sub msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


type Subscription m
    = OnClockTick Time.Time (Time.Time -> m)
    | OnAnimationFrame m
    | OnMouseMove m
    | OnMouseClick m
    | OnKeyPress (Keyboard.KeyCode -> m)
    | OnKeyDown
    | OnKeyUp (Keyboard.KeyCode -> m)
    | OnScrollFromWindowBottom (Scroll.FromBottom -> m)
    | OnWindowResize (Window.Size -> m)
    | FromEventSource ( String, List String ) (EventSource.Msg -> m)
    | OnNewUrl (String -> m)
    | OnTokenReceived (Maybe String -> m)
    | Conditionally Bool (Subscription m)
    | WhenPresent (Maybe (Subscription m))


runSubscription : Subscription Msg -> Sub Msg
runSubscription s =
    case s of
        OnClockTick t m ->
            Time.every t m

        OnAnimationFrame m ->
            AnimationFrame.times (always m)

        OnMouseMove m ->
            Mouse.moves (always m)

        OnMouseClick m ->
            Mouse.clicks (always m)

        OnKeyPress m ->
            Keyboard.presses m

        OnKeyDown ->
            Keyboard.downs Msgs.KeyDown

        OnKeyUp m ->
            Keyboard.ups m

        OnScrollFromWindowBottom m ->
            Scroll.fromWindowBottom m

        OnWindowResize m ->
            Window.resizes m

        FromEventSource key m ->
            EventSource.listen key m

        OnNewUrl m ->
            newUrl m

        OnTokenReceived m ->
            tokenReceived m

        Conditionally True m ->
            runSubscription m

        Conditionally False m ->
            Sub.none

        WhenPresent (Just s) ->
            runSubscription s

        WhenPresent Nothing ->
            Sub.none


map : (m -> n) -> Subscription m -> Subscription n
map f s =
    case s of
        OnClockTick t m ->
            OnClockTick t (m >> f)

        OnAnimationFrame m ->
            OnAnimationFrame (f m)

        OnMouseMove m ->
            OnMouseMove (f m)

        OnMouseClick m ->
            OnMouseClick (f m)

        OnKeyPress m ->
            OnKeyPress (m >> f)

        OnKeyDown ->
            OnKeyDown

        OnKeyUp m ->
            OnKeyUp (m >> f)

        OnScrollFromWindowBottom m ->
            OnScrollFromWindowBottom (m >> f)

        OnWindowResize m ->
            OnWindowResize (m >> f)

        FromEventSource key m ->
            FromEventSource key (m >> f)

        OnNewUrl m ->
            OnNewUrl (m >> f)

        OnTokenReceived m ->
            OnTokenReceived (m >> f)

        Conditionally b m ->
            Conditionally b (map f m)

        WhenPresent (Just s) ->
            WhenPresent (Just (map f s))

        WhenPresent Nothing ->
            WhenPresent Nothing
