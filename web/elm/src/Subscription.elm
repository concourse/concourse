port module Subscription exposing (Delivery(..), Interval(..), Subscription(..), runSubscription)

import AnimationFrame
import Build.StepTree.Models exposing (BuildEventEnvelope)
import Concourse.BuildEvents exposing (decodeBuildEventEnvelope)
import Json.Decode
import Json.Encode
import Keyboard
import Mouse
import Scroll
import Time
import Window


port newUrl : (String -> msg) -> Sub msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


port eventSource : (Json.Encode.Value -> msg) -> Sub msg


type Subscription
    = OnClockTick Interval
    | OnAnimationFrame
    | OnMouse
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
    | Moused
    | ClockTicked Interval Time.Time
    | AnimationFrameAdvanced
    | ScrolledFromWindowBottom Scroll.FromBottom
    | WindowResized Window.Size
    | NonHrefLinkClicked String -- must be a String because we can't parse it out too easily :(
    | TokenReceived (Maybe String)
    | EventReceived (Result String BuildEventEnvelope)


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

        OnMouse ->
            Sub.batch
                [ Mouse.moves (always Moused)
                , Mouse.clicks (always Moused)
                ]

        OnKeyDown ->
            Keyboard.downs KeyDown

        OnKeyUp ->
            Keyboard.ups KeyUp

        OnScrollFromWindowBottom ->
            Scroll.fromWindowBottom ScrolledFromWindowBottom

        OnWindowResize ->
            Window.resizes WindowResized

        FromEventSource key ->
            eventSource
                (Json.Decode.decodeValue decodeBuildEventEnvelope
                    >> EventReceived
                )

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
