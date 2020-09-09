module Job.Job exposing
    ( Flags
    , Model
    , changeToJob
    , documentTitle
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , startingPage
    , subscriptions
    , tooltip
    , update
    , view
    )

import Application.Models exposing (Session)
import Assets
import Colors
import Concourse
import Concourse.BuildStatus exposing (BuildStatus(..))
import Concourse.Pagination
    exposing
        ( Page
        , Paginated
        , chevronContainer
        , chevronLeft
        , chevronRight
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
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (DomID(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import Message.TopLevelMessage exposing (TopLevelMessage(..))
import RemoteData exposing (WebData)
import Routes
import SideBar.SideBar as SideBar
import StrictEvents exposing (onLeftClick)
import Time
import Tooltip
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
        , buildsWithResources : WebData (Paginated BuildWithResources)
        , currentPage : Page
        , now : Time.Posix
        }


type alias BuildWithResources =
    { build : Concourse.Build
    , resources : Maybe Concourse.BuildResources
    }


pageLimit : Int
pageLimit =
    100


type alias Flags =
    { jobId : Concourse.JobIdentifier
    , paging : Maybe Page
    }


startingPage : Page
startingPage =
    { limit = pageLimit
    , direction = Concourse.Pagination.ToMostRecent
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        page =
            flags.paging |> Maybe.withDefault startingPage

        model =
            { jobIdentifier = flags.jobId
            , job = RemoteData.NotAsked
            , pausedChanging = False
            , buildsWithResources = RemoteData.Loading
            , now = Time.millisToPosix 0
            , currentPage = page
            , isUserMenuExpanded = False
            }
    in
    ( model
    , [ FetchJob flags.jobId
      , FetchJobBuilds flags.jobId page
      , GetCurrentTime
      , GetCurrentTimeZone
      , FetchAllPipelines
      ]
    )


changeToJob : Flags -> ET Model
changeToJob flags ( model, effects ) =
    let
        page =
            flags.paging |> Maybe.withDefault startingPage
    in
    ( { model
        | currentPage = page
        , buildsWithResources = RemoteData.Loading
      }
    , effects ++ [ FetchJobBuilds model.jobIdentifier page ]
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
                                            { pipelineId = job.pipelineId
                                            , jobName = job.jobName
                                            , buildName = build.name
                                            }
                                        , highlight = Routes.HighlightNothing
                                        }
                           ]
            )

        JobBuildsFetched (Ok ( requestedPage, builds )) ->
            handleJobBuildsFetched requestedPage builds ( model, effects )

        JobFetched (Ok job) ->
            ( { model | job = RemoteData.Success job }
            , effects
            )

        JobFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | job = RemoteData.Failure err }, effects )

                    else
                        ( model, effects ++ redirectToLoginIfNecessary err )

                _ ->
                    ( model, effects )

        BuildResourcesFetched (Ok ( id, buildResources )) ->
            case model.buildsWithResources of
                RemoteData.Success { content, pagination } ->
                    ( { model
                        | buildsWithResources =
                            RemoteData.Success
                                { content =
                                    List.Extra.updateIf
                                        (\bwr -> bwr.build.id == id)
                                        (\bwr -> { bwr | resources = Just buildResources })
                                        content
                                , pagination = pagination
                                }
                      }
                    , effects
                    )

                _ ->
                    ( model, effects )

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
                ++ [ FetchJobBuilds model.jobIdentifier model.currentPage
                   , FetchJob model.jobIdentifier
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
            { direction = Concourse.Pagination.ToMostRecent
            , limit = pageLimit
            }

        Just build ->
            { direction = Concourse.Pagination.To build.id
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
            case model.buildsWithResources of
                RemoteData.Success bwrs ->
                    List.Extra.find (existingBuild build) bwrs.content

                _ ->
                    Nothing
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


handleJobBuildsFetched : Page -> Paginated Concourse.Build -> ET Model
handleJobBuildsFetched requestedPage paginatedBuilds ( model, effects ) =
    let
        newPage =
            permalink paginatedBuilds.content

        newBWRs =
            setExistingResources paginatedBuilds model
    in
    if
        Concourse.Pagination.isPreviousPage requestedPage
            && (List.length paginatedBuilds.content < pageLimit)
    then
        ( model
        , effects
            ++ [ FetchJobBuilds model.jobIdentifier startingPage
               , NavigateTo <|
                    Routes.toString <|
                        Routes.Job
                            { id = model.jobIdentifier
                            , page = Just startingPage
                            }
               ]
        )

    else
        ( { model
            | buildsWithResources = RemoteData.Success newBWRs
            , currentPage = newPage
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
                , page = Just model.currentPage
                }
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            [ SideBar.hamburgerMenu session
            , TopBar.concourseLogo
            , TopBar.breadcrumbs session route
            , Login.view session.userState model
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view session
                (SideBar.lookupPipeline model.jobIdentifier.pipelineId session)
            , viewMainJobsSection session model
            ]
        ]


tooltip : Model -> a -> Maybe Tooltip.Tooltip
tooltip _ _ =
    Nothing


viewMainJobsSection : Session -> Model -> Html Message
viewMainJobsSection session model =
    let
        archived =
            SideBar.lookupPipeline model.jobIdentifier.pipelineId session
                |> Maybe.map .archived
                |> Maybe.withDefault False
    in
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
                            [ if archived then
                                Html.text ""

                              else
                                Html.button
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
                                            Assets.CircleOutlineIcon <|
                                                if job.paused then
                                                    Assets.PlayCircleIcon

                                                else
                                                    Assets.PauseCircleIcon
                                        }
                                        (Styles.icon toggleHovered)
                                    ]
                            , Html.h1 []
                                [ Html.span
                                    [ class "build-name" ]
                                    [ Html.text job.name ]
                                ]
                            ]
                        , if archived then
                            Html.text ""

                          else
                            Html.button
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
                                    , image = Assets.AddCircleIcon |> Assets.CircleOutlineIcon
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
                            [ style "margin" "0 18px" ]
                            [ Html.text "builds" ]
                        , viewPaginationBar session model
                        ]
                    ]
        , case model.buildsWithResources of
            RemoteData.Success { content } ->
                if List.isEmpty content then
                    Html.div Styles.noBuildsMessage
                        [ Html.text <|
                            "no builds for job “"
                                ++ model.jobIdentifier.jobName
                                ++ "”"
                        ]

                else
                    Html.div
                        [ class "scrollable-body job-body"
                        , style "overflow-y" "auto"
                        ]
                        [ Html.ul [ class "jobs-builds-list builds-list" ] <|
                            List.map (viewBuildWithResources session model) content
                        ]

            _ ->
                LoadingIndicator.view
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
        (case model.buildsWithResources of
            RemoteData.Success { pagination } ->
                [ case pagination.previousPage of
                    Nothing ->
                        Html.div
                            chevronContainer
                            [ Html.div
                                (chevronLeft
                                    { enabled = False
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
                                    ++ chevronLeft
                                        { enabled = True
                                        , hovered =
                                            HoverState.isHovered
                                                PreviousPageButton
                                                session.hovered
                                        }
                                )
                                []
                            ]
                , case pagination.nextPage of
                    Nothing ->
                        Html.div
                            chevronContainer
                            [ Html.div
                                (chevronRight
                                    { enabled = False
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
                                    ++ chevronRight
                                        { enabled = True
                                        , hovered =
                                            HoverState.isHovered
                                                NextPageButton
                                                session.hovered
                                        }
                                )
                                []
                            ]
                ]

            _ ->
                [ Html.div
                    chevronContainer
                    [ Html.div
                        (chevronLeft
                            { enabled = False
                            , hovered = False
                            }
                        )
                        []
                    ]
                , Html.div
                    chevronContainer
                    [ Html.div
                        (chevronRight
                            { enabled = False
                            , hovered = False
                            }
                        )
                        []
                    ]
                ]
        )


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
                , image = Assets.DownArrow
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
                , image = Assets.UpArrow
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
