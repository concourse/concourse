module Build.Header.Views exposing
    ( BackgroundShade(..)
    , BuildDuration(..)
    , ButtonType(..)
    , Header
    , History(..)
    , Timespan(..)
    , Timestamp(..)
    , Widget(..)
    , viewHeader
    )

import Build.Styles as Styles
import Colors
import Concourse
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
        , title
        )
import Html.Events exposing (onBlur, onFocus, onMouseEnter, onMouseLeave)
import Html.Lazy
import Message.Message exposing (DomID(..), Message(..))
import Routes
import StrictEvents exposing (onLeftClick, onMouseWheel)
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
    | Duration BuildDuration


type BuildDuration
    = Pending
    | Running Timestamp
    | Cancelled Timestamp
    | Finished
        { started : Timestamp
        , finished : Timestamp
        , duration : Timespan
        }


type Timestamp
    = Absolute String (Maybe Timespan)
    | Relative Timespan String


type Timespan
    = JustSeconds Int
    | MinutesAndSeconds Int Int
    | HoursAndMinutes Int Int
    | DaysAndHours Int Int


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

        Duration duration ->
            viewDuration duration


viewDuration : BuildDuration -> Html Message
viewDuration duration =
    Html.table [ class "dictionary build-duration" ] <|
        case duration of
            Pending ->
                [ Html.tr []
                    [ Html.td [ class "dict-key" ] [ Html.text "pending" ]
                    , Html.td [ class "dict-value" ] []
                    ]
                ]

            Running timestamp ->
                [ Html.tr []
                    [ Html.td [ class "dict-key" ] [ Html.text "started" ]
                    , viewTimestamp timestamp
                    ]
                ]

            Cancelled timestamp ->
                [ Html.tr []
                    [ Html.td [ class "dict-key" ] [ Html.text "finished" ]
                    , viewTimestamp timestamp
                    ]
                ]

            Finished { started, finished } ->
                [ Html.tr []
                    [ Html.td [ class "dict-key" ] [ Html.text "started" ]
                    , viewTimestamp started
                    ]
                , Html.tr []
                    [ Html.td [ class "dict-key" ] [ Html.text "finished" ]
                    , viewTimestamp finished
                    ]
                ]


viewTimestamp : Timestamp -> Html Message
viewTimestamp timestamp =
    case timestamp of
        Relative timespan formatted ->
            Html.td
                [ class "dict-value"
                , title formatted
                ]
                [ Html.span [] [ Html.text <| viewTimespan timespan ++ " ago" ] ]

        Absolute formatted (Just timespan) ->
            Html.td
                [ class "dict-value"
                , title <| viewTimespan timespan
                ]
                [ Html.span [] [ Html.text formatted ] ]

        Absolute formatted Nothing ->
            Html.td
                [ class "dict-value"
                ]
                [ Html.span [] [ Html.text formatted ] ]


viewTimespan : Timespan -> String
viewTimespan timespan =
    case timespan of
        JustSeconds s ->
            String.fromInt s ++ "s"

        MinutesAndSeconds m s ->
            String.fromInt m ++ "m" ++ String.fromInt s ++ "s"

        HoursAndMinutes h m ->
            String.fromInt h ++ "h" ++ String.fromInt m ++ "m"

        DaysAndHours d h ->
            String.fromInt d ++ "d" ++ String.fromInt h ++ "h"


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
