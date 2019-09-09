module Build.Header.Views exposing
    ( BackgroundShade(..)
    , ButtonType(..)
    , Header
    , History(..)
    , Widget(..)
    , viewHeader
    )

import Build.Styles as Styles
import Colors
import Concourse exposing (BuildDuration)
import Concourse.BuildStatus exposing (BuildStatus)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , classList
        , href
        , id
        , style
        )
import Html.Events exposing (onBlur, onFocus, onMouseEnter, onMouseLeave)
import Html.Lazy
import Message.Message exposing (DomID(..), Message(..))
import Routes
import StrictEvents exposing (onLeftClick, onMouseWheel)
import Time
import Views.BuildDuration as BuildDuration
import Views.Icon as Icon


historyId : String
historyId =
    "builds"


type alias Header =
    { leftWidgets : List Widget
    , rightWidgets : List Widget
    , backgroundColor : BuildStatus
    , history : History
    }


type Widget
    = Button (Maybe ButtonView)
    | Title String (Maybe Concourse.JobIdentifier)
    | Duration Time.Zone BuildDuration (Maybe Time.Posix) -- TODO this type should not depend on time zone or current time, it should just contain timestamps


type History
    = History Concourse.Build (List Concourse.Build)


type alias ButtonView =
    { type_ : ButtonType
    , isClickable : Bool
    , backgroundShade : BackgroundShade
    , backgroundColor : BuildStatus
    , tooltip : Bool
    }


type BackgroundShade
    = Light
    | Dark


type ButtonType
    = Abort
    | Trigger


viewHeader : Header -> Html Message
viewHeader header =
    Html.div [ class "fixed-header" ]
        [ Html.div
            ([ id "build-header"
             , class "build-header"
             ]
                ++ Styles.header header.backgroundColor
            )
            [ Html.div [] (List.map viewWidget header.leftWidgets)
            , Html.div [ style "display" "flex" ] (List.map viewWidget header.rightWidgets)
            ]
        , viewHistory header.history
        ]


viewWidget : Widget -> Html Message
viewWidget widget =
    case widget of
        Button button ->
            Maybe.map viewButton button |> Maybe.withDefault (Html.text "")

        Title name jobId ->
            Html.h1 [] [ viewTitle name jobId ]

        Duration timeZone duration now ->
            Maybe.map (BuildDuration.view timeZone duration) now
                |> Maybe.withDefault (Html.text "")


lazyViewHistory : History -> Html Message
lazyViewHistory history =
    Html.Lazy.lazy viewHistoryItems history


viewHistoryItems : History -> Html Message
viewHistoryItems (History currentBuild builds) =
    Html.ul [ id historyId ]
        (List.map (viewHistoryItem currentBuild) builds)


viewHistoryItem : Concourse.Build -> Concourse.Build -> Html Message
viewHistoryItem currentBuild build =
    Html.li
        ([ classList [ ( "current", build.id == currentBuild.id ) ]
         , id <| String.fromInt build.id
         ]
            ++ Styles.historyItem build.status
        )
        [ Html.a
            [ onLeftClick <| Click <| BuildTab build
            , href <| Routes.toString <| Routes.buildRoute build
            ]
            [ Html.text build.name ]
        ]


viewButton : ButtonView -> Html Message
viewButton { type_, tooltip, backgroundColor, backgroundShade, isClickable } =
    let
        image =
            case type_ of
                Abort ->
                    "ic-abort-circle-outline-white.svg"

                Trigger ->
                    "ic-add-circle-outline-white.svg"

        accessibilityLabel =
            case type_ of
                Abort ->
                    "Abort Build"

                Trigger ->
                    "Trigger Build"

        domID =
            case type_ of
                Abort ->
                    AbortBuildButton

                Trigger ->
                    TriggerBuildButton

        styles =
            [ style "padding" "10px"
            , style "outline" "none"
            , style "margin" "0"
            , style "border-width" "0 0 0 1px"
            , style "border-color" Colors.background
            , style "border-style" "solid"
            , style "position" "relative"
            , style "background-color" <|
                Colors.buildStatusColor
                    (backgroundShade == Light)
                    backgroundColor
            , style "cursor" <|
                if isClickable then
                    "pointer"

                else
                    "default"
            ]
    in
    Html.button
        ([ attribute "role" "button"
         , attribute "tabindex" "0"
         , attribute "aria-label" accessibilityLabel
         , attribute "title" accessibilityLabel
         , onLeftClick <| Click domID
         , onMouseEnter <| Hover <| Just domID
         , onFocus <| Hover <| Just domID
         , onMouseLeave <| Hover Nothing
         , onBlur <| Hover Nothing
         ]
            ++ styles
        )
        [ Icon.icon
            { sizePx = 40
            , image = image
            }
            []
        , viewTooltip tooltip type_
        ]


viewTooltip : Bool -> ButtonType -> Html Message
viewTooltip tooltip type_ =
    case ( tooltip, type_ ) of
        ( True, Trigger ) ->
            Html.div
                Styles.triggerTooltip
                [ Html.text <|
                    "manual triggering disabled in job config"
                ]

        _ ->
            Html.text ""


viewTitle : String -> Maybe Concourse.JobIdentifier -> Html Message
viewTitle name jobID =
    case jobID of
        Just id ->
            Html.a
                [ href <|
                    Routes.toString <|
                        Routes.Job { id = id, page = Nothing }
                ]
                [ Html.span [ class "build-name" ] [ Html.text id.jobName ]
                , Html.text (" #" ++ name)
                ]

        _ ->
            Html.text ("build #" ++ name)


viewHistory : History -> Html Message
viewHistory history =
    Html.div
        [ onMouseWheel ScrollBuilds ]
        [ lazyViewHistory history ]
