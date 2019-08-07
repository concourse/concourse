module Build.Header.Header exposing (handleDelivery, update, viewBuildHeader)

import Application.Models exposing (Session)
import Build.Header.Models exposing (Model)
import Build.Styles as Styles
import Concourse
import Concourse.BuildStatus
import EffectTransformer exposing (ET)
import HoverState
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
import List.Extra
import Message.Effects as Effects exposing (Effect(..), ScrollDirection(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import RemoteData
import Routes
import StrictEvents exposing (onLeftClick, onMouseWheel)
import Views.BuildDuration as BuildDuration
import Views.Icon as Icon


historyId : String
historyId =
    "builds"


viewBuildHeader :
    Session
    -> Model r
    -> Concourse.Build
    -> Html Message
viewBuildHeader session model build =
    let
        triggerButton =
            case currentJob model of
                Just _ ->
                    let
                        buttonDisabled =
                            model.disableManualTrigger

                        buttonHovered =
                            HoverState.isHovered
                                TriggerBuildButton
                                session.hovered
                    in
                    Html.button
                        ([ attribute "role" "button"
                         , attribute "tabindex" "0"
                         , attribute "aria-label" "Trigger Build"
                         , attribute "title" "Trigger Build"
                         , onLeftClick <| Click TriggerBuildButton
                         , onMouseEnter <| Hover <| Just TriggerBuildButton
                         , onFocus <| Hover <| Just TriggerBuildButton
                         , onMouseLeave <| Hover Nothing
                         , onBlur <| Hover Nothing
                         ]
                            ++ Styles.triggerButton
                                buttonDisabled
                                buttonHovered
                                build.status
                        )
                    <|
                        [ Icon.icon
                            { sizePx = 40
                            , image = "ic-add-circle-outline-white.svg"
                            }
                            []
                        ]
                            ++ (if buttonDisabled && buttonHovered then
                                    [ Html.div
                                        Styles.triggerTooltip
                                        [ Html.text <|
                                            "manual triggering disabled "
                                                ++ "in job config"
                                        ]
                                    ]

                                else
                                    []
                               )

                Nothing ->
                    Html.text ""

        abortHovered =
            HoverState.isHovered AbortBuildButton session.hovered

        abortButton =
            if Concourse.BuildStatus.isRunning build.status then
                Html.button
                    ([ onLeftClick (Click <| AbortBuildButton)
                     , attribute "role" "button"
                     , attribute "tabindex" "0"
                     , attribute "aria-label" "Abort Build"
                     , attribute "title" "Abort Build"
                     , onMouseEnter <| Hover <| Just AbortBuildButton
                     , onFocus <| Hover <| Just AbortBuildButton
                     , onMouseLeave <| Hover Nothing
                     , onBlur <| Hover Nothing
                     ]
                        ++ Styles.abortButton abortHovered
                    )
                    [ Icon.icon
                        { sizePx = 40
                        , image = "ic-abort-circle-outline-white.svg"
                        }
                        []
                    ]

            else
                Html.text ""

        buildTitle =
            case build.job of
                Just jobId ->
                    let
                        jobRoute =
                            Routes.Job { id = jobId, page = Nothing }
                    in
                    Html.a
                        [ href <| Routes.toString jobRoute ]
                        [ Html.span [ class "build-name" ] [ Html.text jobId.jobName ]
                        , Html.text (" #" ++ build.name)
                        ]

                _ ->
                    Html.text ("build #" ++ String.fromInt build.id)
    in
    Html.div [ class "fixed-header" ]
        [ Html.div
            ([ id "build-header"
             , class "build-header"
             ]
                ++ Styles.header build.status
            )
            [ Html.div []
                [ Html.h1 [] [ buildTitle ]
                , case model.now of
                    Just n ->
                        BuildDuration.view session.timeZone build.duration n

                    Nothing ->
                        Html.text ""
                ]
            , Html.div
                [ style "display" "flex" ]
                [ abortButton, triggerButton ]
            ]
        , Html.div
            [ onMouseWheel ScrollBuilds ]
            [ lazyViewHistory build model.history ]
        ]


currentJob : Model r -> Maybe Concourse.JobIdentifier
currentJob =
    .currentBuild
        >> RemoteData.toMaybe
        >> Maybe.map .build
        >> Maybe.andThen .job


lazyViewHistory : Concourse.Build -> List Concourse.Build -> Html Message
lazyViewHistory currentBuild builds =
    Html.Lazy.lazy2 viewHistory currentBuild builds


viewHistory : Concourse.Build -> List Concourse.Build -> Html Message
viewHistory currentBuild builds =
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


handleDelivery : Delivery -> ET (Model r)
handleDelivery delivery ( model, effects ) =
    case delivery of
        ElementVisible ( id, False ) ->
            let
                currentBuildInvisible =
                    model.currentBuild
                        |> RemoteData.toMaybe
                        |> Maybe.map (.build >> .id >> String.fromInt)
                        |> Maybe.map ((==) id)
                        |> Maybe.withDefault False

                shouldScroll =
                    currentBuildInvisible && not model.scrolledToCurrentBuild
            in
            ( { model | scrolledToCurrentBuild = True }
            , effects
                ++ (if shouldScroll then
                        [ Scroll (ToId id) historyId ]

                    else
                        []
                   )
            )

        _ ->
            ( model, effects )


update : Message -> ET (Model r)
update msg ( model, effects ) =
    case msg of
        ScrollBuilds event ->
            let
                scroll =
                    if event.deltaX == 0 then
                        [ Scroll (Sideways event.deltaY) historyId ]

                    else
                        [ Scroll (Sideways -event.deltaX) historyId ]

                checkVisibility =
                    case model.history |> List.Extra.last of
                        Just build ->
                            [ Effects.CheckIsVisible <| String.fromInt build.id ]

                        Nothing ->
                            []
            in
            ( model, effects ++ scroll ++ checkVisibility )

        _ ->
            ( model, effects )
