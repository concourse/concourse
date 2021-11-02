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

import Assets
import Build.Styles as Styles
import Colors
import Concourse
import Concourse.BuildStatus exposing (BuildStatus)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , href
        , id
        , style
        , title
        )
import Html.Events exposing (onBlur, onFocus, onMouseEnter, onMouseLeave)
import Html.Lazy
import Message.Effects exposing (toHtmlID)
import Message.Message as Message exposing (Message(..))
import Routes
import StrictEvents exposing (onLeftClick, onWheel)
import Views.CommentBar as CommentBar
import Views.Icon as Icon


historyId : String
historyId =
    "builds"


type alias Header =
    { leftWidgets : List Widget
    , rightWidgets : List Widget
    , backgroundColor : BuildStatus
    , tabs : List BuildTab
    , comment : Maybe ( CommentBar.Model, CommentBar.ViewState )
    }


type Widget
    = Button (Maybe ButtonView)
    | Title String (Maybe Concourse.JobIdentifier) Concourse.BuildCreatedBy
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
    , hasComment : Bool
    }


type alias ButtonView =
    { type_ : ButtonType
    , isClickable : Bool
    , backgroundShade : BackgroundShade
    , backgroundColor : BuildStatus
    }


type BackgroundShade
    = Light
    | Dark


type ButtonType
    = Abort
    | ToggleComment
    | Trigger
    | Rerun


viewHeader : Header -> Html Message
viewHeader header =
    Html.div [ class "fixed-header" ]
        ([ Html.div
            ([ id "build-header"
             , class "build-header"
             ]
                ++ Styles.header header.backgroundColor
            )
            [ Html.div [] (List.map viewWidget header.leftWidgets)
            , Html.div [ style "display" "flex" ] (List.map viewWidget header.rightWidgets)
            ]
         , viewHistory header.backgroundColor header.tabs
         ]
            ++ (case header.comment of
                    Nothing ->
                        []

                    Just ( model, state ) ->
                        [ Html.div
                            [ id (toHtmlID model.id)
                            , style "display" "flex"
                            ]
                            [ Html.div [ style "flex" "1 1 0%" ] []
                            , CommentBar.view [ style "flex" "2 1 0%" ] state model
                            , Html.div [ style "flex" "1 1 0%" ] []
                            ]
                        ]
               )
        )


viewWidget : Widget -> Html Message
viewWidget widget =
    case widget of
        Button button ->
            Maybe.map viewButton button |> Maybe.withDefault (Html.text "")

        Title name jobId createdBy ->
            viewTitle name jobId createdBy

        Duration duration ->
            viewDuration duration


viewDuration : BuildDuration -> Html Message
viewDuration buildDuration =
    Html.table
        [ class "dictionary build-duration"
        , style "color" Colors.black
        ]
        [ Html.tr []
            [ Html.td [ class "horizontal-cell" ] <|
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
            , Html.td [ class "horizontal-cell" ] <|
                case buildDuration of
                    Finished { duration } ->
                        [ Html.tr []
                            [ Html.td [ class "dict-key" ] [ Html.text "duration" ]
                            , Html.td [ class "dict-value" ] [ Html.text <| viewTimespan duration ]
                            ]
                        ]

                    _ ->
                        []
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


lazyViewHistory : BuildStatus -> List BuildTab -> Html Message
lazyViewHistory backgroundColor =
    Html.Lazy.lazy (viewBuildTabs backgroundColor)


viewBuildTabs : BuildStatus -> List BuildTab -> Html Message
viewBuildTabs backgroundColor =
    List.map (viewBuildTab backgroundColor) >> Html.ul [ id historyId ]


viewBuildTab : BuildStatus -> BuildTab -> Html Message
viewBuildTab backgroundColor tab =
    Html.li
        (class "history-item"
            :: (id <| toHtmlID <| Message.BuildTab tab.id tab.name)
            :: Styles.historyItem backgroundColor tab.isCurrent tab.background
        )
        ((if tab.hasComment then
            [ Html.div (Styles.historyTriangle "5px") [] ]

          else
            []
         )
            ++ [ Html.a
                    [ style "color" <| Colors.buildTabTextColor tab.isCurrent tab.background
                    , onLeftClick <| Click <| Message.BuildTab tab.id tab.name
                    , onMouseEnter <| Hover <| Just <| Message.BuildTab tab.id tab.name
                    , onMouseLeave <| Hover Nothing
                    , href <| Routes.toString tab.href
                    ]
                    [ Html.text tab.name ]
               ]
        )


viewButton : ButtonView -> Html Message
viewButton { type_, backgroundColor, backgroundShade, isClickable } =
    let
        image =
            case type_ of
                ToggleComment ->
                    Assets.MessageIcon

                Abort ->
                    Assets.AbortCircleIcon |> Assets.CircleOutlineIcon

                Trigger ->
                    Assets.AddCircleIcon |> Assets.CircleOutlineIcon

                Rerun ->
                    Assets.RerunIcon

        accessibilityLabel =
            case type_ of
                ToggleComment ->
                    "Toggle Build Comment"

                Abort ->
                    "Abort Build"

                Trigger ->
                    "Trigger Build"

                Rerun ->
                    "Rerun Build"

        domID =
            case type_ of
                ToggleComment ->
                    Message.ToggleBuildCommentButton

                Abort ->
                    Message.AbortBuildButton

                Trigger ->
                    Message.TriggerBuildButton

                Rerun ->
                    Message.RerunBuildButton

        styles =
            [ style "padding" "16px"
            , style "outline" "none"
            , style "margin" "0"
            , style "border-width" "0 0 0 1px"
            , style "border-color" <| Colors.buildStatusColor False backgroundColor
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
         , onMouseEnter <| Hover <| Just domID
         , onMouseLeave <| Hover Nothing
         , onFocus <| Hover <| Just domID
         , onBlur <| Hover Nothing
         , id <| toHtmlID domID
         ]
            ++ (if isClickable then
                    [ onLeftClick <| Click domID ]

                else
                    []
               )
            ++ styles
        )
        [ Icon.icon
            { sizePx = 28
            , image = image
            }
            []
        ]


viewTitle : String -> Maybe Concourse.JobIdentifier -> Concourse.BuildCreatedBy -> Html Message
viewTitle name jobID createdBy =
    let
        hasCreatedBy =
            createdBy /= Nothing

        buildName =
            let
                buildNameLineHeight =
                    style "line-height" <|
                        if hasCreatedBy then
                            "44px"

                        else
                            "60px"
            in
            case jobID of
                Just jid ->
                    Html.a
                        [ href <|
                            Routes.toString <|
                                Routes.Job { id = jid, page = Nothing, groups = [] }
                        , onMouseEnter <| Hover <| Just Message.JobName
                        , onMouseLeave <| Hover Nothing
                        , id <| toHtmlID Message.JobName
                        , buildNameLineHeight
                        , style "color" Colors.buildTitleTextColor
                        ]
                        [ Html.span [ class "build-name" ] [ Html.text jid.jobName ]
                        , Html.span
                            [ style "letter-spacing" "-1px"
                            , style "margin-left" "16px"
                            ]
                            [ Html.text ("#" ++ name) ]
                        ]

                Nothing ->
                    Html.span [ buildNameLineHeight ] [ Html.text name ]

        createdByText =
            case createdBy of
                Just who ->
                    let
                        text =
                            "created by " ++ who
                    in
                    Html.span
                        [ style "position" "absolute"
                        , style "font-size" "12px"
                        , style "bottom" "6px"
                        , style "line-height" "16px"
                        , style "right" "0"
                        , style "left" "0"
                        , style "text-overflow" "ellipsis"
                        , style "white-space" "nowrap"
                        , style "overflow" "hidden"
                        , style "color" Colors.buildTitleTextColor
                        , title text
                        ]
                        [ Html.text text ]

                Nothing ->
                    Html.text ""

        headerStyle =
            [ style "position" "relative"
            , style "height" "60px"
            , style "line-height"
                (if hasCreatedBy then
                    "38px"

                 else
                    "60px"
                )
            ]
                ++ (if hasCreatedBy then
                        [ style "min-width" "100px" ]

                    else
                        []
                   )
    in
    Html.h1 headerStyle [ buildName, createdByText ]


viewHistory : BuildStatus -> List BuildTab -> Html Message
viewHistory backgroundColor =
    lazyViewHistory backgroundColor
        >> List.singleton
        >> Html.div [ onWheel ScrollBuilds ]
