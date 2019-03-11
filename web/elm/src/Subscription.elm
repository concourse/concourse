port module Subscription exposing
    ( Delivery(..)
    , Interval(..)
    , Subscription(..)
    , runSubscription
    )

import Build.StepTree.Models exposing (BuildEventEnvelope)
import Concourse.BuildEvents exposing (decodeBuildEventEnvelope)
import Json.Decode
import Json.Encode
import Keyboard
import Mouse
import Time
import Window


port newUrl : (String -> msg) -> Sub msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


port eventSource : (Json.Encode.Value -> msg) -> Sub msg


port scrolledToBottom : (Bool -> msg) -> Sub msg


port reportIsVisible : (( String, Bool ) -> msg) -> Sub msg


type Subscription
    = OnClockTick Interval
    | OnMouse
    | OnKeyDown
    | OnKeyUp
    | OnScrollToBottom
    | OnWindowResize
    | FromEventSource ( String, List String )
    | OnNonHrefLinkClicked
    | OnTokenReceived
    | OnElementVisible


type Delivery
    = KeyDown Keyboard.KeyCode
    | KeyUp Keyboard.KeyCode
    | Moused
    | ClockTicked Interval Time.Time
    | ScrolledToBottom Bool
    | WindowResized Window.Size
    | NonHrefLinkClicked String -- must be a String because we can't parse it out too easily :(
    | TokenReceived (Maybe String)
    | EventsReceived (Result String (List BuildEventEnvelope))
    | ElementVisible ( String, Bool )


type Interval
    = OneSecond
    | FiveSeconds
    | OneMinute


runSubscription : Subscription -> Sub Delivery
runSubscription s =
    case s of
        OnClockTick t ->
            Time.every (intervalToTime t) (ClockTicked t)

        OnMouse ->
            Sub.batch
                [ Mouse.moves (always Moused)
                , Mouse.clicks (always Moused)
                ]

        OnKeyDown ->
            Keyboard.downs KeyDown

        OnKeyUp ->
            Keyboard.ups KeyUp

        OnScrollToBottom ->
            scrolledToBottom ScrolledToBottom

        OnWindowResize ->
            Window.resizes WindowResized

        FromEventSource key ->
            eventSource
                (Json.Decode.decodeValue
                    (Json.Decode.list decodeBuildEventEnvelope)
                    >> EventsReceived
                )

        OnNonHrefLinkClicked ->
            newUrl NonHrefLinkClicked

        OnTokenReceived ->
            tokenReceived TokenReceived

        OnElementVisible ->
            reportIsVisible ElementVisible


intervalToTime : Interval -> Time.Time
intervalToTime t =
    case t of
        OneSecond ->
            Time.second

        FiveSeconds ->
            5 * Time.second

        OneMinute ->
            Time.minute
