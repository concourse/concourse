module Job.Job exposing
    ( Flags
    , Model
    , changeToJob
    , documentTitle
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    )

import Application.Models exposing (Session)
import Colors
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Pagination
    exposing
        ( Page
        , Paginated
        , chevron
        , chevronContainer
        )
import Dict
import EffectTransformer exposing (ET)
import HoverState
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , href
        , id
        , style
        )
import Html.Events
    exposing
        ( onClick
        , onMouseEnter
        , onMouseLeave
        )
import Http
import Job.Styles as Styles
import Login.Login as Login
import Message.Callback as Callback
    exposing
        ( Callback(..)
        , HttpMethod(..)
        , Route(..)
        )
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import RemoteData exposing (WebData)
import Routes
import SideBar.SideBar as SideBar
import StrictEvents exposing (onLeftClick)
import Time
import UpdateMsg exposing (UpdateMsg)
import Views.BuildDuration as BuildDuration
import Views.DictView as DictView
import Views.Icon as Icon
import Views.LoadingIndicator as LoadingIndicator
import Views.Styles
import Views.TopBar as TopBar


type alias Model =
    Login.Model
        { jobIdentifier : Concourse.JobIdentifier
        , job : WebData Concourse.Job
        , pausedChanging : Bool
        , buildsWithResources : Paginated BuildWithResources
        , currentPage : Maybe Page
        , now : Time.Posix
        }


type alias BuildWithResources =
    { build : Concourse.Build
    , resources : Maybe Concourse.BuildResources
    }


jobBuildsPerPage : Int
jobBuildsPerPage =
    100


type alias Flags =
    { jobId : Concourse.JobIdentifier
    , paging : Maybe Page
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        model =
            { jobIdentifier = flags.jobId
            , job = RemoteData.NotAsked
            , pausedChanging = False
            , buildsWithResources =
                { content = []
                , pagination =
                    { previousPage = Nothing
                    , nextPage = Nothing
                    }
                }
            , now = Time.millisToPosix 0
            , currentPage = flags.paging
            , isUserMenuExpanded = False
            }
    in
    ( model
    , [ ApiCall (RouteJob flags.jobId) GET
      , ApiCall (RouteJobBuilds flags.jobId flags.paging) GET
      , GetCurrentTime
      , GetCurrentTimeZone
      , FetchAllPipelines
      ]
    )


changeToJob : Flags -> ET Model
changeToJob flags ( model, effects ) =
    ( { model
        | currentPage = flags.paging
        , buildsWithResources =
            { content = []
            , pagination =
                { previousPage = Nothing
                , nextPage = Nothing
                }
            }
      }
    , effects
        ++ [ ApiCall (RouteJobBuilds model.jobIdentifier flags.paging) GET ]
    )


subscriptions : List Subscription
subscriptions =
    [ OnClockTick FiveSeconds
    , OnClockTick OneSecond
    ]


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    case model.job of
        RemoteData.Failure _ ->
            UpdateMsg.NotFound

        _ ->
            UpdateMsg.AOK


handleCallback : Callback -> ET Model
handleCallback callback ( model, effects ) =
    case callback of
        BuildTriggered (Ok build) ->
            ( model
            , case build.job of
                Nothing ->
                    effects

                Just job ->
                    effects
                        ++ [ NavigateTo <|
                                Routes.toString <|
                                    Routes.Build
                                        { id =
                                            { teamName = job.teamName
                                            , pipelineName = job.pipelineName
                                            , jobName = job.jobName
                                            , buildName = build.name
                                            }
                                        , highlight = Routes.HighlightNothing
                                        }
                           ]
            )

        ApiResponse (RouteJobBuilds _ _) (Ok (Callback.Builds builds)) ->
            handleJobBuildsFetched builds ( model, effects )

        ApiResponse (RouteJob _) (Ok (Callback.Job job)) ->
            ( { model | job = RemoteData.Success job }
            , effects
            )

        ApiResponse (RouteJob _) (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | job = RemoteData.Failure err }, effects )

                    else
                        ( model, effects ++ redirectToLoginIfNecessary err )

                _ ->
                    ( model, effects )

        BuildResourcesFetched (Ok ( id, buildResources )) ->
            case model.buildsWithResources.content of
                [] ->
                    ( model, effects )

                anyList ->
                    let
                        transformer bwr =
                            if bwr.build.id == id then
                                { bwr | resources = Just buildResources }

                            else
                                bwr

                        bwrs =
                            model.buildsWithResources
                    in
                    ( { model
                        | buildsWithResources =
                            { bwrs
                                | content = List.map transformer anyList
                            }
                      }
                    , effects
                    )

        BuildResourcesFetched (Err _) ->
            ( model, effects )

        PausedToggled (Ok ()) ->
            ( { model | pausedChanging = False }, effects )

        GotCurrentTime now ->
            ( { model | now = now }, effects )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        ClockTicked OneSecond time ->
            ( { model | now = time }, effects )

        ClockTicked FiveSeconds _ ->
            ( model
            , effects
                ++ [ ApiCall
                        (RouteJobBuilds model.jobIdentifier model.currentPage)
                        GET
                   , ApiCall (RouteJob model.jobIdentifier) GET
                   , FetchAllPipelines
                   ]
            )

        _ ->
            ( model, effects )


update : Message -> ET Model
update action ( model, effects ) =
    case action of
        Click TriggerBuildButton ->
            ( model, effects ++ [ DoTriggerBuild model.jobIdentifier ] )

        Click ToggleJobButton ->
            case model.job |> RemoteData.toMaybe of
                Nothing ->
                    ( model, effects )

                Just j ->
                    ( { model
                        | pausedChanging = True
                        , job = RemoteData.Success { j | paused = not j.paused }
                      }
                    , if j.paused then
                        effects ++ [ UnpauseJob model.jobIdentifier ]

                      else
                        effects ++ [ PauseJob model.jobIdentifier ]
                    )

        _ ->
            ( model, effects )


redirectToLoginIfNecessary : Http.Error -> List Effect
redirectToLoginIfNecessary err =
    case err of
        Http.BadStatus { status } ->
            if status.code == 401 then
                [ RedirectToLogin ]

            else
                []

        _ ->
            []


permalink : List Concourse.Build -> Page
permalink builds =
    case List.head builds of
        Nothing ->
            { direction = Concourse.Pagination.Since 0
            , limit = jobBuildsPerPage
            }

        Just build ->
            { direction = Concourse.Pagination.Since (build.id + 1)
            , limit = List.length builds
            }


paginatedMap : (a -> b) -> Paginated a -> Paginated b
paginatedMap promoter pagA =
    { content =
        List.map promoter pagA.content
    , pagination = pagA.pagination
    }


setResourcesToOld : Maybe BuildWithResources -> BuildWithResources -> BuildWithResources
setResourcesToOld existingBuildWithResource newBwr =
    case existingBuildWithResource of
        Nothing ->
            newBwr

        Just buildWithResources ->
            { newBwr
                | resources = buildWithResources.resources
            }


existingBuild : Concourse.Build -> BuildWithResources -> Bool
existingBuild build buildWithResources =
    build == buildWithResources.build


promoteBuild : Model -> Concourse.Build -> BuildWithResources
promoteBuild model build =
    let
        newBwr =
            { build = build
            , resources = Nothing
            }

        existingBuildWithResource =
            List.head
                (List.filter (existingBuild build) model.buildsWithResources.content)
    in
    setResourcesToOld existingBuildWithResource newBwr


setExistingResources : Paginated Concourse.Build -> Model -> Paginated BuildWithResources
setExistingResources paginatedBuilds model =
    paginatedMap (promoteBuild model) paginatedBuilds


updateResourcesIfNeeded : BuildWithResources -> Maybe Effect
updateResourcesIfNeeded bwr =
    case ( bwr.resources, isRunning bwr.build ) of
        ( Just _, False ) ->
            Nothing

        _ ->
            Just <| FetchBuildResources bwr.build.id


handleJobBuildsFetched : Paginated Concourse.Build -> ET Model
handleJobBuildsFetched paginatedBuilds ( model, effects ) =
    let
        newPage =
            permalink paginatedBuilds.content

        newBWRs =
            setExistingResources paginatedBuilds model
    in
    ( { model
        | buildsWithResources = newBWRs
        , currentPage = Just newPage
      }
    , effects ++ List.filterMap updateResourcesIfNeeded newBWRs.content
    )


isRunning : Concourse.Build -> Bool
isRunning build =
    Concourse.BuildStatus.isRunning build.status


documentTitle : Model -> String
documentTitle model =
    model.jobIdentifier.jobName


view : Session -> Model -> Html Message
view session model =
    let
        route =
            Routes.Job
                { id = model.jobIdentifier
                , page = model.currentPage
                }
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            [ SideBar.hamburgerMenu session
            , TopBar.concourseLogo
            , TopBar.breadcrumbs route
            , Login.view session.userState model False
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view
                { expandedTeams = session.expandedTeams
                , pipelines = session.pipelines
                , hovered = session.hovered
                , isSideBarOpen = session.isSideBarOpen
                , screenSize = session.screenSize
                }
                (Just
                    { pipelineName = model.jobIdentifier.pipelineName
                    , teamName = model.jobIdentifier.teamName
                    }
                )
            , viewMainJobsSection session model
            ]
        ]


viewMainJobsSection : Session -> Model -> Html Message
viewMainJobsSection session model =
    Html.div
        [ class "with-fixed-header"
        , style "flex-grow" "1"
        , style "display" "flex"
        , style "flex-direction" "column"
        ]
        [ case model.job |> RemoteData.toMaybe of
            Nothing ->
                LoadingIndicator.view

            Just job ->
                let
                    toggleHovered =
                        HoverState.isHovered ToggleJobButton session.hovered

                    triggerHovered =
                        HoverState.isHovered TriggerBuildButton session.hovered
                in
                Html.div [ class "fixed-header" ]
                    [ Html.div
                        [ class "build-header"
                        , style "display" "flex"
                        , style "justify-content" "space-between"
                        , style "background" <|
                            Colors.buildStatusColor True <|
                                headerBuildStatus job.finishedBuild
                        ]
                        [ Html.div
                            [ style "display" "flex" ]
                            [ Html.button
                                ([ id "pause-toggle"
                                 , onMouseEnter <| Hover <| Just ToggleJobButton
                                 , onMouseLeave <| Hover Nothing
                                 , onClick <| Click ToggleJobButton
                                 ]
                                    ++ (Styles.triggerButton False toggleHovered <|
                                            headerBuildStatus job.finishedBuild
                                       )
                                )
                                [ Icon.icon
                                    { sizePx = 40
                                    , image =
                                        if job.paused then
                                            "ic-play-circle-outline.svg"

                                        else
                                            "ic-pause-circle-outline-white.svg"
                                    }
                                    (Styles.icon toggleHovered)
                                ]
                            , Html.h1 []
                                [ Html.span
                                    [ class "build-name" ]
                                    [ Html.text job.name ]
                                ]
                            ]
                        , Html.button
                            ([ class "trigger-build"
                             , onLeftClick <| Click TriggerBuildButton
                             , attribute "aria-label" "Trigger Build"
                             , attribute "title" "Trigger Build"
                             , onMouseEnter <| Hover <| Just TriggerBuildButton
                             , onMouseLeave <| Hover Nothing
                             ]
                                ++ (Styles.triggerButton job.disableManualTrigger triggerHovered <|
                                        headerBuildStatus job.finishedBuild
                                   )
                            )
                          <|
                            [ Icon.icon
                                { sizePx = 40
                                , image = "ic-add-circle-outline-white.svg"
                                }
                                (Styles.icon <|
                                    triggerHovered
                                        && not job.disableManualTrigger
                                )
                            ]
                                ++ (if job.disableManualTrigger && triggerHovered then
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
                        ]
                    , Html.div
                        [ id "pagination-header"
                        , style "display" "flex"
                        , style "justify-content" "space-between"
                        , style "align-items" "stretch"
                        , style "height" "60px"
                        , style "background-color" Colors.secondaryTopBar
                        ]
                        [ Html.h1
                            [ style "margin" "0 18px"
                            , style "font-weight" "700"
                            ]
                            [ Html.text "builds" ]
                        , viewPaginationBar session model
                        ]
                    ]
        , case ( model.buildsWithResources.content, model.currentPage ) of
            ( _, Nothing ) ->
                LoadingIndicator.view

            ( [], Just _ ) ->
                Html.div Styles.noBuildsMessage
                    [ Html.text <|
                        "no builds for job “"
                            ++ model.jobIdentifier.jobName
                            ++ "”"
                    ]

            ( anyList, Just _ ) ->
                Html.div
                    [ class "scrollable-body job-body"
                    , style "overflow-y" "auto"
                    ]
                    [ Html.ul [ class "jobs-builds-list builds-list" ] <|
                        List.map (viewBuildWithResources session model) anyList
                    ]
        ]


headerBuildStatus : Maybe Concourse.Build -> BuildStatus
headerBuildStatus finishedBuild =
    case finishedBuild of
        Nothing ->
            BuildStatusPending

        Just build ->
            build.status


viewPaginationBar : { a | hovered : HoverState.HoverState } -> Model -> Html Message
viewPaginationBar session model =
    Html.div
        [ id "pagination"
        , style "display" "flex"
        , style "align-items" "stretch"
        ]
        [ case model.buildsWithResources.pagination.previousPage of
            Nothing ->
                Html.div
                    chevronContainer
                    [ Html.div
                        (chevron
                            { direction = "left"
                            , enabled = False
                            , hovered = False
                            }
                        )
                        []
                    ]

            Just page ->
                let
                    jobRoute =
                        Routes.Job { id = model.jobIdentifier, page = Just page }
                in
                Html.div
                    ([ onMouseEnter <| Hover <| Just PreviousPageButton
                     , onMouseLeave <| Hover Nothing
                     ]
                        ++ chevronContainer
                    )
                    [ Html.a
                        ([ StrictEvents.onLeftClick <| GoToRoute jobRoute
                         , href <| Routes.toString <| jobRoute
                         , attribute "aria-label" "Previous Page"
                         ]
                            ++ chevron
                                { direction = "left"
                                , enabled = True
                                , hovered =
                                    HoverState.isHovered
                                        PreviousPageButton
                                        session.hovered
                                }
                        )
                        []
                    ]
        , case model.buildsWithResources.pagination.nextPage of
            Nothing ->
                Html.div
                    chevronContainer
                    [ Html.div
                        (chevron
                            { direction = "right"
                            , enabled = False
                            , hovered = False
                            }
                        )
                        []
                    ]

            Just page ->
                let
                    jobRoute =
                        Routes.Job { id = model.jobIdentifier, page = Just page }
                in
                Html.div
                    ([ onMouseEnter <| Hover <| Just NextPageButton
                     , onMouseLeave <| Hover Nothing
                     ]
                        ++ chevronContainer
                    )
                    [ Html.a
                        ([ StrictEvents.onLeftClick <| GoToRoute jobRoute
                         , href <| Routes.toString jobRoute
                         , attribute "aria-label" "Next Page"
                         ]
                            ++ chevron
                                { direction = "right"
                                , enabled = True
                                , hovered =
                                    HoverState.isHovered
                                        NextPageButton
                                        session.hovered
                                }
                        )
                        []
                    ]
        ]


viewBuildWithResources :
    Session
    -> Model
    -> BuildWithResources
    -> Html Message
viewBuildWithResources session model bwr =
    Html.li [ class "js-build" ] <|
        let
            buildResourcesView =
                viewBuildResources bwr
        in
        [ viewBuildHeader bwr.build
        , Html.div [ class "pam clearfix" ] <|
            BuildDuration.view session.timeZone bwr.build.duration model.now
                :: buildResourcesView
        ]


viewBuildHeader : Concourse.Build -> Html Message
viewBuildHeader b =
    Html.a
        [ class <| Concourse.BuildStatus.show b.status
        , StrictEvents.onLeftClick <|
            GoToRoute <|
                Routes.buildRoute b.id b.name b.job
        , href <|
            Routes.toString <|
                Routes.buildRoute b.id b.name b.job
        ]
        [ Html.text ("#" ++ b.name)
        ]


viewBuildResources : BuildWithResources -> List (Html Message)
viewBuildResources buildWithResources =
    let
        inputsTable =
            case buildWithResources.resources of
                Nothing ->
                    LoadingIndicator.view

                Just resources ->
                    Html.table [ class "build-resources" ] <|
                        List.map viewBuildInputs resources.inputs

        outputsTable =
            case buildWithResources.resources of
                Nothing ->
                    LoadingIndicator.view

                Just resources ->
                    Html.table [ class "build-resources" ] <|
                        List.map viewBuildOutputs resources.outputs
    in
    [ Html.div [ class "inputs mrl" ]
        [ Html.div
            Styles.buildResourceHeader
            [ Icon.icon
                { sizePx = 12
                , image = "ic-arrow-downward.svg"
                }
                Styles.buildResourceIcon
            , Html.text "inputs"
            ]
        , inputsTable
        ]
    , Html.div [ class "outputs mrl" ]
        [ Html.div
            Styles.buildResourceHeader
            [ Icon.icon
                { sizePx = 12
                , image = "ic-arrow-upward.svg"
                }
                Styles.buildResourceIcon
            , Html.text "outputs"
            ]
        , outputsTable
        ]
    ]


viewBuildInputs : Concourse.BuildResourcesInput -> Html Message
viewBuildInputs bi =
    Html.tr [ class "mbs pas resource fl clearfix" ]
        [ Html.td [ class "resource-name mrm" ]
            [ Html.text bi.name
            ]
        , Html.td [ class "resource-version" ]
            [ viewVersion bi.version
            ]
        ]


viewBuildOutputs : Concourse.BuildResourcesOutput -> Html Message
viewBuildOutputs bo =
    Html.tr [ class "mbs pas resource fl clearfix" ]
        [ Html.td [ class "resource-name mrm" ]
            [ Html.text bo.name
            ]
        , Html.td [ class "resource-version" ]
            [ viewVersion bo.version
            ]
        ]


viewVersion : Concourse.Version -> Html Message
viewVersion version =
    version
        |> Dict.map (always Html.text)
        |> DictView.view []
