port module Message.Subscription exposing
    ( Delivery(..)
    , Interval(..)
    , Subscription(..)
    , runSubscription
    )

import Browser
import Browser.Events exposing (onClick, onKeyDown, onKeyUp, onMouseMove, onResize)
import Build.StepTree.Models exposing (BuildEventEnvelope)
import Concourse.BuildEvents exposing (decodeBuildEventEnvelope)
import Json.Decode
import Json.Encode
import Keyboard
import Routes
import Time
import Url


port newUrl : (String -> msg) -> Sub msg


port tokenReceived : (Maybe String -> msg) -> Sub msg


port eventSource : (Json.Encode.Value -> msg) -> Sub msg


port reportIsVisible : (( String, Bool ) -> msg) -> Sub msg


port sideBarStateReceived : (Maybe String -> msg) -> Sub msg


type Subscription
    = OnClockTick Interval
    | OnMouse
    | OnKeyDown
    | OnKeyUp
    | OnWindowResize
    | FromEventSource ( String, List String )
    | OnNonHrefLinkClicked
    | OnTokenReceived
    | OnElementVisible
    | OnSideBarStateReceived


type Delivery
    = KeyDown Keyboard.KeyEvent
    | KeyUp Keyboard.KeyEvent
    | Moused
    | ClockTicked Interval Time.Posix
    | WindowResized Float Float
    | NonHrefLinkClicked String -- must be a String because we can't parse it out too easily :(
    | TokenReceived (Maybe String)
    | EventsReceived (Result Json.Decode.Error (List BuildEventEnvelope))
    | RouteChanged Routes.Route
    | UrlRequest Browser.UrlRequest
    | ElementVisible ( String, Bool )
    | SideBarStateReceived (Maybe String)


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
                [ onMouseMove (Json.Decode.succeed Moused)
                , onClick (Json.Decode.succeed Moused)
                ]

        OnKeyDown ->
            onKeyDown (Keyboard.decodeKeyEvent |> Json.Decode.map KeyDown)

        OnKeyUp ->
            onKeyUp (Keyboard.decodeKeyEvent |> Json.Decode.map KeyUp)

        OnWindowResize ->
            onResize
                (\width height -> WindowResized (toFloat width) (toFloat height))

        FromEventSource _ ->
            eventSource
                (Json.Decode.decodeValue
                    (Json.Decode.list decodeBuildEventEnvelope)
                    >> EventsReceived
                )

        OnNonHrefLinkClicked ->
            newUrl
                (\path ->
                    let
                        url =
                            { protocol = Url.Http
                            , host = ""
                            , port_ = Nothing
                            , path = path
                            , query = Nothing
                            , fragment = Nothing
                            }
                    in
                    case Routes.parsePath url of
                        Just _ ->
                            UrlRequest <| Browser.Internal url

                        Nothing ->
                            UrlRequest <| Browser.External path
                )

        OnTokenReceived ->
            tokenReceived TokenReceived

        OnElementVisible ->
            reportIsVisible ElementVisible

        OnSideBarStateReceived ->
            sideBarStateReceived SideBarStateReceived


intervalToTime : Interval -> Float
intervalToTime t =
    case t of
        OneSecond ->
            1000

        FiveSeconds ->
            5 * 1000

        OneMinute ->
            60 * 1000
