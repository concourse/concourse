module Build.Header.Views exposing
    ( BackgroundShade(..)
    , BuildDuration(..)
    , BuildTab
    , ButtonType(..)
    , Header
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
import Message.Message as Message exposing (Message(..))
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
    , tabs : List BuildTab
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


type alias BuildTab =
    { id : Int
    , name : String
    , background : BuildStatus
    , href : Routes.Route
    , isCurrent : Bool
    }


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
    | Rerun


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
        , viewHistory header.tabs
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
viewDuration buildDuration =
    Html.table [ class "dictionary build-duration" ] <|
        case buildDuration of
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

            Finished { started, finished, duration } ->
                [ Html.tr []
                    [ Html.td [ class "dict-key" ] [ Html.text "started" ]
                    , viewTimestamp started
                    ]
                , Html.tr []
                    [ Html.td [ class "dict-key" ] [ Html.text "finished" ]
                    , viewTimestamp finished
                    ]
                , Html.tr []
                    [ Html.td [ class "dict-key" ] [ Html.text "duration" ]
                    , Html.td [ class "dict-value" ] [ Html.text <| viewTimespan duration ]
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
            String.fromInt m ++ "m " ++ String.fromInt s ++ "s"

        HoursAndMinutes h m ->
            String.fromInt h ++ "h " ++ String.fromInt m ++ "m"

        DaysAndHours d h ->
            String.fromInt d ++ "d " ++ String.fromInt h ++ "h"


lazyViewHistory : List BuildTab -> Html Message
lazyViewHistory =
    Html.Lazy.lazy viewBuildTabs


viewBuildTabs : List BuildTab -> Html Message
viewBuildTabs =
    List.map viewBuildTab >> Html.ul [ id historyId ]


viewBuildTab : BuildTab -> Html Message
viewBuildTab tab =
    Html.li
        ([ classList [ ( "current", tab.isCurrent ) ]
         , id <| String.fromInt tab.id
         ]
            ++ Styles.historyItem tab.background
        )
        [ Html.a
            [ onLeftClick <| Click <| Message.BuildTab tab.id tab.name
            , href <| Routes.toString tab.href
            ]
            [ Html.text tab.name ]
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

                Rerun ->
                    "ic-rerun.svg"

        accessibilityLabel =
            case type_ of
                Abort ->
                    "Abort Build"

                Trigger ->
                    "Trigger Build"

                Rerun ->
                    "Rerun Build"

        domID =
            case type_ of
                Abort ->
                    Message.AbortBuildButton

                Trigger ->
                    Message.TriggerBuildButton

                Rerun ->
                    Message.RerunBuildButton

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
         , onMouseLeave <| Hover Nothing
         , onFocus <| Hover <| Just domID
         , onBlur <| Hover Nothing
         ]
            ++ styles
        )
        [ Icon.icon
            { sizePx = 40
            , image = image
            }
            []
        , tooltipArrow tooltip type_
        , viewTooltip tooltip type_
        ]


tooltipArrow : Bool -> ButtonType -> Html Message
tooltipArrow tooltip type_ =
    case ( tooltip, type_ ) of
        ( True, Trigger ) ->
            Html.div
                Styles.buttonTooltipArrow
                []

        ( True, Rerun ) ->
            Html.div
                Styles.buttonTooltipArrow
                []

        _ ->
            Html.text ""


viewTooltip : Bool -> ButtonType -> Html Message
viewTooltip tooltip type_ =
    case ( tooltip, type_ ) of
        ( True, Trigger ) ->
            Html.div
                (Styles.buttonTooltip 240)
                [ Html.text <|
                    "manual triggering disabled in job config"
                ]

        ( True, Rerun ) ->
            Html.div
                (Styles.buttonTooltip 165)
                [ Html.text <|
                    "re-run with the same inputs"
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


viewHistory : List BuildTab -> Html Message
viewHistory =
    lazyViewHistory >> List.singleton >> Html.div [ onMouseWheel ScrollBuilds ]
