module Job.Job exposing
    ( Flags
    , Model
    , changeToJob
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    )

import Colors
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination
    exposing
        ( Page
        , Paginated
        , Pagination
        , chevron
        , chevronContainer
        )
import Dict exposing (Dict)
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , disabled
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
import Message.Message exposing (Hoverable(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import RemoteData exposing (WebData)
import Routes
import StrictEvents exposing (onLeftClick)
import Time
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState)
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
        , hovered : Maybe Hoverable
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
            , hovered = Nothing
            , isUserMenuExpanded = False
            }
    in
    ( model
    , [ FetchJob flags.jobId
      , FetchJobBuilds flags.jobId flags.paging
      , GetCurrentTime
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
    , effects ++ [ FetchJobBuilds model.jobIdentifier flags.paging ]
    )


subscriptions : Model -> List Subscription
subscriptions model =
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

        JobBuildsFetched (Ok builds) ->
            handleJobBuildsFetched builds ( model, effects )

        JobFetched (Ok job) ->
            ( { model | job = RemoteData.Success job }
            , effects ++ [ SetTitle <| job.name ++ " - " ]
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

        BuildResourcesFetched (Err err) ->
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

        ClockTicked FiveSeconds time ->
            ( model
            , effects
                ++ [ FetchJobBuilds model.jobIdentifier model.currentPage
                   , FetchJob model.jobIdentifier
                   ]
            )

        _ ->
            ( model, effects )


update : Message -> ET Model
update action ( model, effects ) =
    case action of
        TriggerBuildJob ->
            ( model, effects ++ [ DoTriggerBuild model.jobIdentifier ] )

        TogglePaused ->
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

        Hover hoverable ->
            ( { model | hovered = hoverable }, effects )

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
        ( Just resources, False ) ->
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


view : UserState -> Model -> Html Message
view userState model =
    Html.div []
        [ Html.div
            ([ id "page-including-top-bar" ] ++ Views.Styles.pageIncludingTopBar)
            [ Html.div
                ([ id "top-bar-app" ] ++ Views.Styles.topBar False)
                [ TopBar.concourseLogo
                , TopBar.breadcrumbs <|
                    Routes.Job
                        { id = model.jobIdentifier
                        , page = model.currentPage
                        }
                , Login.view userState model False
                ]
            , Html.div
                ([ id "page-below-top-bar" ] ++ Styles.pageBelowTopBar)
                [ viewMainJobsSection model ]
            ]
        ]


viewMainJobsSection : Model -> Html Message
viewMainJobsSection model =
    Html.div [ class "with-fixed-header" ]
        [ case model.job |> RemoteData.toMaybe of
            Nothing ->
                LoadingIndicator.view

            Just job ->
                let
                    toggleHovered =
                        model.hovered == Just ToggleJobButton

                    triggerHovered =
                        model.hovered == Just TriggerBuildButton
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
                                 , onClick TogglePaused
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
                             , onLeftClick TriggerBuildJob
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
                        , viewPaginationBar model
                        ]
                    ]
        , case model.buildsWithResources.content of
            [] ->
                LoadingIndicator.view

            anyList ->
                Html.div [ class "scrollable-body job-body" ]
                    [ Html.ul [ class "jobs-builds-list builds-list" ] <|
                        List.map (viewBuildWithResources model) anyList
                    ]
        ]


headerBuildStatus : Maybe Concourse.Build -> Concourse.BuildStatus
headerBuildStatus finishedBuild =
    case finishedBuild of
        Nothing ->
            Concourse.BuildStatusPending

        Just build ->
            build.status


viewPaginationBar : Model -> Html Message
viewPaginationBar model =
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
                                , hovered = model.hovered == Just PreviousPageButton
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
                                , hovered = model.hovered == Just NextPageButton
                                }
                        )
                        []
                    ]
        ]


viewBuildWithResources : Model -> BuildWithResources -> Html Message
viewBuildWithResources model bwr =
    Html.li [ class "js-build" ] <|
        let
            buildResourcesView =
                viewBuildResources model bwr
        in
        [ viewBuildHeader model bwr.build
        , Html.div [ class "pam clearfix" ] <|
            BuildDuration.view bwr.build.duration model.now
                :: buildResourcesView
        ]


viewBuildHeader : Model -> Concourse.Build -> Html Message
viewBuildHeader model b =
    Html.a
        [ class <| Concourse.BuildStatus.show b.status
        , StrictEvents.onLeftClick <| GoToRoute <| Routes.buildRoute b
        , href <| Routes.toString <| Routes.buildRoute b
        ]
        [ Html.text ("#" ++ b.name)
        ]


viewBuildResources : Model -> BuildWithResources -> List (Html Message)
viewBuildResources model buildWithResources =
    let
        inputsTable =
            case buildWithResources.resources of
                Nothing ->
                    LoadingIndicator.view

                Just resources ->
                    Html.table [ class "build-resources" ] <|
                        List.map (viewBuildInputs model) resources.inputs

        outputsTable =
            case buildWithResources.resources of
                Nothing ->
                    LoadingIndicator.view

                Just resources ->
                    Html.table [ class "build-resources" ] <|
                        List.map (viewBuildOutputs model) resources.outputs
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


viewBuildInputs : Model -> Concourse.BuildResourcesInput -> Html Message
viewBuildInputs model bi =
    Html.tr [ class "mbs pas resource fl clearfix" ]
        [ Html.td [ class "resource-name mrm" ]
            [ Html.text bi.name
            ]
        , Html.td [ class "resource-version" ]
            [ viewVersion bi.version
            ]
        ]


viewBuildOutputs : Model -> Concourse.BuildResourcesOutput -> Html Message
viewBuildOutputs model bo =
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
    DictView.view []
        << Dict.map (\_ s -> Html.text s)
    <|
        version
