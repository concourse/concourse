port module Message.Subscription exposing
    ( Delivery(..)
    , Interval(..)
    , RawHttpResponse(..)
    , Subscription(..)
    , decodeHttpResponse
    , runSubscription
    )

import Browser
import Browser.Events
    exposing
        ( onClick
        , onKeyDown
        , onKeyUp
        , onMouseMove
        , onMouseUp
        , onResize
        )
import Build.StepTree.Models exposing (BuildEventEnvelope)
import Concourse exposing (DatabaseID, decodeInstanceGroupId, decodeJob, decodePipeline, decodeTeam)
import Concourse.BuildEvents exposing (decodeBuildEventEnvelope)
import Json.Decode
import Json.Encode
import Keyboard
import Message.Storage as Storage
    exposing
        ( favoritedInstanceGroupsKey
        , favoritedPipelinesKey
        , jobsKey
        , pipelinesKey
        , receivedFromCache
        , receivedFromLocalStorage
        , sideBarStateKey
        , teamsKey
        , tokenKey
        )
import Routes
import Set exposing (Set)
import SideBar.State exposing (SideBarState, decodeSideBarState)
import Time
import Url


port newUrl : (String -> msg) -> Sub msg


port eventSource : (Json.Encode.Value -> msg) -> Sub msg


port reportIsVisible : (( String, Bool ) -> msg) -> Sub msg


port rawHttpResponse : (String -> msg) -> Sub msg


port scrolledToId : (( String, String ) -> msg) -> Sub msg


type alias Position =
    { x : Float
    , y : Float
    }


type alias DatabaseID =
    Int


type RawHttpResponse
    = Success
    | Timeout
    | NetworkError
    | BrowserError


type Subscription
    = OnClockTick Interval
    | OnMouse
    | OnMouseUp
    | OnKeyDown
    | OnKeyUp
    | OnWindowResize
    | FromEventSource ( String, List String )
    | OnNonHrefLinkClicked
    | OnElementVisible
    | OnTokenSentToFly
    | OnLocalStorageReceived
    | OnCacheReceived
    | OnScrolledToId


type Delivery
    = KeyDown Keyboard.KeyEvent
    | KeyUp Keyboard.KeyEvent
    | Moused Position
    | MouseUp
    | ClockTicked Interval Time.Posix
    | WindowResized Float Float
    | NonHrefLinkClicked String -- must be a String because we can't parse it out too easily :(
    | EventsReceived (Result Json.Decode.Error (List BuildEventEnvelope))
    | RouteChanged Routes.Route
    | UrlRequest Browser.UrlRequest
    | ElementVisible ( String, Bool )
    | TokenSentToFly RawHttpResponse
    | TokenReceived (Result Json.Decode.Error String)
    | SideBarStateReceived (Result Json.Decode.Error SideBarState)
    | CachedJobsReceived (Result Json.Decode.Error (List Concourse.Job))
    | CachedPipelinesReceived (Result Json.Decode.Error (List Concourse.Pipeline))
    | CachedTeamsReceived (Result Json.Decode.Error (List Concourse.Team))
    | FavoritedPipelinesReceived (Result Json.Decode.Error (Set DatabaseID))
    | FavoritedInstanceGroupsReceived (Result Json.Decode.Error (Set ( Concourse.TeamName, Concourse.PipelineName )))
    | ScrolledToId ( String, String )
    | Noop


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
                [ onMouseMove (Json.Decode.map Moused decodePosition)
                , onClick (Json.Decode.map Moused decodePosition)
                ]

        OnMouseUp ->
            onMouseUp <| Json.Decode.succeed MouseUp

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

        OnLocalStorageReceived ->
            receivedFromLocalStorage <|
                decodeStorageResponse

        OnCacheReceived ->
            receivedFromCache <|
                decodeStorageResponse

        OnElementVisible ->
            reportIsVisible ElementVisible

        OnTokenSentToFly ->
            rawHttpResponse (decodeHttpResponse >> TokenSentToFly)

        OnScrolledToId ->
            scrolledToId ScrolledToId


decodePosition : Json.Decode.Decoder Position
decodePosition =
    Json.Decode.map2 Position
        (Json.Decode.field "pageX" Json.Decode.float)
        (Json.Decode.field "pageY" Json.Decode.float)


decodeStorageResponse : ( Storage.Key, Storage.Value ) -> Delivery
decodeStorageResponse ( key, value ) =
    let
        decodeValue decoder toDelivery =
            Json.Decode.decodeValue decoder >> toDelivery
    in
    value
        |> (if key == tokenKey then
                decodeValue Json.Decode.string TokenReceived

            else if key == sideBarStateKey then
                decodeValue decodeSideBarState SideBarStateReceived

            else if key == jobsKey then
                decodeValue (Json.Decode.list decodeJob) CachedJobsReceived

            else if key == pipelinesKey then
                decodeValue (Json.Decode.list decodePipeline) CachedPipelinesReceived

            else if key == teamsKey then
                decodeValue (Json.Decode.list decodeTeam) CachedTeamsReceived

            else if key == favoritedPipelinesKey then
                decodeValue (Json.Decode.list Json.Decode.int |> Json.Decode.map Set.fromList)
                    FavoritedPipelinesReceived

            else if key == favoritedInstanceGroupsKey then
                decodeValue
                    (Json.Decode.list decodeInstanceGroupId
                        |> Json.Decode.map
                            (List.map (\{ teamName, name } -> ( teamName, name ))
                                >> Set.fromList
                            )
                    )
                    FavoritedInstanceGroupsReceived

            else
                always Noop
           )


decodeHttpResponse : String -> RawHttpResponse
decodeHttpResponse value =
    case value of
        "networkError" ->
            NetworkError

        "browserError" ->
            BrowserError

        "timeout" ->
            Timeout

        _ ->
            Success


intervalToTime : Interval -> Float
intervalToTime t =
    case t of
        OneSecond ->
            1000

        FiveSeconds ->
            5 * 1000

        OneMinute ->
            60 * 1000
