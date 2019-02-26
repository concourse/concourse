port module Subscription exposing (Subscription(..), map, runSubscription)

import AnimationFrame
import Application.Msgs as Msgs exposing (Msg(..))
import EventSource.EventSource as EventSource
import Keyboard
import Mouse
import Scroll
import Time
import Window


port newUrl : (String -> msg) -> Sub msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


type Subscription m
    = OnClockTick Time.Time (Time.Time -> m)
    | OnAnimationFrame m
    | OnMouseMove
    | OnMouseClick
    | OnKeyDown
    | OnKeyUp
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

        OnMouseMove ->
            Mouse.moves (always (Msgs.DeliveryReceived Msgs.MouseMoved))

        OnMouseClick ->
            Mouse.clicks (always (Msgs.DeliveryReceived Msgs.MouseClicked))

        OnKeyDown ->
            Keyboard.downs (Msgs.DeliveryReceived << Msgs.KeyDown)

        OnKeyUp ->
            Keyboard.ups (Msgs.DeliveryReceived << Msgs.KeyUp)

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

        OnMouseMove ->
            OnMouseMove

        OnMouseClick ->
            OnMouseClick

        OnKeyDown ->
            OnKeyDown

        OnKeyUp ->
            OnKeyUp

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

        WhenPresent s ->
            WhenPresent (Maybe.map (map f) s)
