port module Subscription exposing (Subscription(..), map, runSubscription)

import AnimationFrame
import Application.Msgs as Msgs exposing (Interval, Msg(..))
import EventSource.EventSource as EventSource
import Keyboard
import Mouse
import Scroll
import Time
import Window


port newUrl : (String -> msg) -> Sub msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


type Subscription m
    = OnClockTick Interval
    | OnAnimationFrame
    | OnMouseMove
    | OnMouseClick
    | OnKeyDown
    | OnKeyUp
    | OnScrollFromWindowBottom
    | OnWindowResize
    | FromEventSource ( String, List String ) (EventSource.Msg -> m)
    | OnNonHrefLinkClicked
    | OnTokenReceived
    | WhenPresent (Maybe (Subscription m))


runSubscription : Subscription Msg -> Sub Msg
runSubscription s =
    case s of
        OnClockTick t ->
            Time.every (Msgs.intervalToTime t) (Msgs.DeliveryReceived << Msgs.ClockTicked t)

        OnAnimationFrame ->
            AnimationFrame.times (always (Msgs.DeliveryReceived Msgs.AnimationFrameAdvanced))

        OnMouseMove ->
            Mouse.moves (always (Msgs.DeliveryReceived Msgs.MouseMoved))

        OnMouseClick ->
            Mouse.clicks (always (Msgs.DeliveryReceived Msgs.MouseClicked))

        OnKeyDown ->
            Keyboard.downs (Msgs.DeliveryReceived << Msgs.KeyDown)

        OnKeyUp ->
            Keyboard.ups (Msgs.DeliveryReceived << Msgs.KeyUp)

        OnScrollFromWindowBottom ->
            Scroll.fromWindowBottom (Msgs.DeliveryReceived << Msgs.ScrolledFromWindowBottom)

        OnWindowResize ->
            Window.resizes (Msgs.DeliveryReceived << Msgs.WindowResized)

        FromEventSource key m ->
            EventSource.listen key m

        OnNonHrefLinkClicked ->
            newUrl (Msgs.DeliveryReceived << Msgs.NonHrefLinkClicked)

        OnTokenReceived ->
            tokenReceived (Msgs.DeliveryReceived << Msgs.TokenReceived)

        WhenPresent (Just s) ->
            runSubscription s

        WhenPresent Nothing ->
            Sub.none


map : (m -> n) -> Subscription m -> Subscription n
map f s =
    case s of
        OnClockTick t ->
            OnClockTick t

        OnAnimationFrame ->
            OnAnimationFrame

        OnMouseMove ->
            OnMouseMove

        OnMouseClick ->
            OnMouseClick

        OnKeyDown ->
            OnKeyDown

        OnKeyUp ->
            OnKeyUp

        OnScrollFromWindowBottom ->
            OnScrollFromWindowBottom

        OnWindowResize ->
            OnWindowResize

        FromEventSource key m ->
            FromEventSource key (m >> f)

        OnNonHrefLinkClicked ->
            OnNonHrefLinkClicked

        OnTokenReceived ->
            OnTokenReceived

        WhenPresent s ->
            WhenPresent (Maybe.map (map f) s)
