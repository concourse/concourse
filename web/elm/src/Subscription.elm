port module Subscription exposing (Delivery(..), Interval(..), Subscription(..), runSubscription)

import AnimationFrame
import EventSource.EventSource as EventSource
import Keyboard
import Mouse
import Scroll
import Time
import Window


port newUrl : (String -> msg) -> Sub msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


type Subscription
    = OnClockTick Interval
    | OnAnimationFrame
    | OnMouseMove
    | OnMouseClick
    | OnKeyDown
    | OnKeyUp
    | OnScrollFromWindowBottom
    | OnWindowResize
    | FromEventSource ( String, List String )
    | OnNonHrefLinkClicked
    | OnTokenReceived


type Delivery
    = KeyDown Keyboard.KeyCode
    | KeyUp Keyboard.KeyCode
    | MouseMoved
    | MouseClicked
    | ClockTicked Interval Time.Time
    | AnimationFrameAdvanced
    | ScrolledFromWindowBottom Scroll.FromBottom
    | WindowResized Window.Size
    | NonHrefLinkClicked String -- must be a String because we can't parse it out too easily :(
    | TokenReceived (Maybe String)
    | EventReceived EventSource.Msg


type Interval
    = OneSecond
    | FiveSeconds
    | OneMinute


runSubscription : Subscription -> Sub Delivery
runSubscription s =
    case s of
        OnClockTick t ->
            Time.every (intervalToTime t) (ClockTicked t)

        OnAnimationFrame ->
            AnimationFrame.times (always AnimationFrameAdvanced)

        OnMouseMove ->
            Mouse.moves (always MouseMoved)

        OnMouseClick ->
            Mouse.clicks (always MouseClicked)

        OnKeyDown ->
            Keyboard.downs KeyDown

        OnKeyUp ->
            Keyboard.ups KeyUp

        OnScrollFromWindowBottom ->
            Scroll.fromWindowBottom ScrolledFromWindowBottom

        OnWindowResize ->
            Window.resizes WindowResized

        FromEventSource key ->
            EventSource.listen key EventReceived

        OnNonHrefLinkClicked ->
            newUrl NonHrefLinkClicked

        OnTokenReceived ->
            tokenReceived TokenReceived


intervalToTime : Interval -> Time.Time
intervalToTime t =
    case t of
        OneSecond ->
            Time.second

        FiveSeconds ->
            5 * Time.second

        OneMinute ->
            Time.minute
