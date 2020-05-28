module Dashboard.Dashboard exposing
    ( documentTitle
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Application.Models exposing (Session)
import Concourse
import Concourse.Cli as Cli
import Dashboard.DashboardPreview as DashboardPreview
import Dashboard.Drag as Drag
import Dashboard.Filter as Filter
import Dashboard.Footer as Footer
import Dashboard.Group as Group
import Dashboard.Group.Models exposing (Pipeline)
import Dashboard.Models as Models
    exposing
        ( DragState(..)
        , DropState(..)
        , Dropdown(..)
        , FetchError(..)
        , Model
        )
import Dashboard.PipelineGrid as PipelineGrid
import Dashboard.PipelineGrid.Constants as PipelineGridConstants
import Dashboard.RequestBuffer as RequestBuffer exposing (Buffer(..))
import Dashboard.SearchBar as SearchBar
import Dashboard.Styles as Styles
import Dashboard.Text as Text
import Dict exposing (Dict)
import EffectTransformer exposing (ET)
import FetchResult exposing (FetchResult(..), changedFrom)
import HoverState
import Html exposing (Html)
import Html.Attributes
    exposing
        ( attribute
        , class
        , download
        , href
        , id
        , src
        , style
        )
import Html.Events
    exposing
        ( onMouseEnter
        , onMouseLeave
        )
import Http
import List.Extra
import Login.Login as Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..), toHtmlID)
import Message.Message as Message
    exposing
        ( DomID(..)
        , Message(..)
        , VisibilityAction(..)
        )
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Routes
import ScreenSize exposing (ScreenSize(..))
import SideBar.SideBar as SideBar
import StrictEvents exposing (onScroll)
import Time
import Tooltip
import UserState
import Views.Spinner as Spinner
import Views.Styles


init : Routes.SearchType -> ( Model, List Effect )
init searchType =
    ( { now = Nothing
      , hideFooter = False
      , hideFooterCounter = 0
      , showHelp = False
      , highDensity = searchType == Routes.HighDensity
      , query = Routes.extractQuery searchType
      , pipelinesWithResourceErrors = Dict.empty
      , jobs = None
      , pipelines = None
      , pipelineLayers = Dict.empty
      , teams = None
      , isUserMenuExpanded = False
      , dropdown = Hidden
      , dragState = Models.NotDragging
      , dropState = Models.NotDropping
      , isJobsRequestFinished = False
      , isTeamsRequestFinished = False
      , isResourcesRequestFinished = False
      , isPipelinesRequestFinished = False
      , jobsError = Nothing
      , teamsError = Nothing
      , resourcesError = Nothing
      , pipelinesError = Nothing
      , viewportWidth = 0
      , viewportHeight = 0
      , scrollTop = 0
      , pipelineJobs = Dict.empty
      , effectsToRetry = []
      }
    , [ FetchAllTeams
      , PinTeamNames Message.Effects.stickyHeaderConfig
      , GetScreenSize
      , FetchAllResources
      , FetchAllJobs
      , FetchAllPipelines
      , LoadCachedJobs
      , LoadCachedPipelines
      , LoadCachedTeams
      , GetViewportOf Dashboard
      ]
    )


buffers : List (Buffer Model)
buffers =
    [ Buffer FetchAllTeams
        (\c ->
            case c of
                AllTeamsFetched _ ->
                    True

                _ ->
                    False
        )
        (.dragState >> (/=) NotDragging)
        { get = \m -> m.isTeamsRequestFinished
        , set = \f m -> { m | isTeamsRequestFinished = f }
        }
    , Buffer FetchAllResources
        (\c ->
            case c of
                AllResourcesFetched _ ->
                    True

                _ ->
                    False
        )
        (.dragState >> (/=) NotDragging)
        { get = \m -> m.isResourcesRequestFinished
        , set = \f m -> { m | isResourcesRequestFinished = f }
        }
    , Buffer FetchAllJobs
        (\c ->
            case c of
                AllJobsFetched _ ->
                    True

                _ ->
                    False
        )
        (\model -> model.dragState /= NotDragging || model.jobsError == Just Disabled)
        { get = \m -> m.isJobsRequestFinished
        , set = \f m -> { m | isJobsRequestFinished = f }
        }
    , Buffer FetchAllPipelines
        (\c ->
            case c of
                AllPipelinesFetched _ ->
                    True

                _ ->
                    False
        )
        (.dragState >> (/=) NotDragging)
        { get = \m -> m.isPipelinesRequestFinished
        , set = \f m -> { m | isPipelinesRequestFinished = f }
        }
    ]


handleCallback : Callback -> ET Model
handleCallback callback ( model, effects ) =
    (case callback of
        AllTeamsFetched (Err _) ->
            ( { model | teamsError = Just Failed }
            , effects
            )

        AllTeamsFetched (Ok teams) ->
            let
                newTeams =
                    Fetched teams
            in
            ( { model
                | teams = newTeams
                , teamsError = Nothing
              }
            , effects
                ++ (if newTeams |> changedFrom model.teams then
                        [ SaveCachedTeams teams ]

                    else
                        []
                   )
            )

        AllJobsFetched (Ok allJobsInEntireCluster) ->
            let
                removeBuild job =
                    { job
                        | finishedBuild = Nothing
                        , transitionBuild = Nothing
                        , nextBuild = Nothing
                    }

                newJobs =
                    allJobsInEntireCluster
                        |> List.map
                            (\job ->
                                ( ( job.teamName
                                  , job.pipelineName
                                  , job.name
                                  )
                                , job
                                )
                            )
                        |> Dict.fromList
                        |> Fetched

                maxJobsInCache =
                    1000

                mapToJobIds jobsResult =
                    jobsResult
                        |> FetchResult.map (Dict.toList >> List.map Tuple.first)

                newModel =
                    { model
                        | jobs = newJobs
                        , jobsError = Nothing
                    }
            in
            if mapToJobIds newJobs |> changedFrom (mapToJobIds model.jobs) then
                ( newModel |> precomputeJobMetadata
                , effects
                    ++ [ allJobsInEntireCluster
                            |> List.take maxJobsInCache
                            |> List.map removeBuild
                            |> SaveCachedJobs
                       ]
                )

            else
                ( newModel, effects )

        AllJobsFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    case status.code of
                        501 ->
                            ( { model
                                | jobsError = Just Disabled
                                , jobs = Fetched Dict.empty
                                , pipelines =
                                    model.pipelines
                                        |> FetchResult.map
                                            (List.map
                                                (\p ->
                                                    { p | jobsDisabled = True }
                                                )
                                            )
                              }
                            , effects ++ [ DeleteCachedJobs ]
                            )

                        503 ->
                            ( { model
                                | effectsToRetry =
                                    model.effectsToRetry
                                        ++ (if List.member FetchAllJobs model.effectsToRetry then
                                                []

                                            else
                                                [ FetchAllJobs ]
                                           )
                              }
                            , effects
                            )

                        _ ->
                            ( { model | jobsError = Just Failed }, effects )

                _ ->
                    ( { model | jobsError = Just Failed }, effects )

        AllResourcesFetched (Ok resources) ->
            ( { model
                | pipelinesWithResourceErrors =
                    resources
                        |> List.foldr
                            (\r ->
                                Dict.update ( r.teamName, r.pipelineName )
                                    (Maybe.withDefault False
                                        >> (||) r.failingToCheck
                                        >> Just
                                    )
                            )
                            model.pipelinesWithResourceErrors
                , resourcesError = Nothing
              }
            , effects
            )

        AllResourcesFetched (Err _) ->
            ( { model | resourcesError = Just Failed }, effects )

        AllPipelinesFetched (Ok allPipelinesInEntireCluster) ->
            let
                newPipelines =
                    allPipelinesInEntireCluster
                        |> List.map (toDashboardPipeline False (model.jobsError == Just Disabled))
                        |> Fetched
            in
            ( { model
                | pipelines = newPipelines
                , pipelinesError = Nothing
              }
            , effects
                ++ (if List.isEmpty allPipelinesInEntireCluster then
                        [ ModifyUrl "/" ]

                    else
                        []
                   )
                ++ (if newPipelines |> pipelinesChangedFrom model.pipelines then
                        [ SaveCachedPipelines allPipelinesInEntireCluster ]

                    else
                        []
                   )
            )

        AllPipelinesFetched (Err _) ->
            ( { model | pipelinesError = Just Failed }, effects )

        PipelinesOrdered teamName _ ->
            ( model, effects ++ [ FetchPipelines teamName ] )

        PipelinesFetched _ ->
            ( { model | dropState = NotDropping }
            , effects
            )

        LoggedOut (Ok ()) ->
            ( model
            , effects
                ++ [ NavigateTo <|
                        Routes.toString <|
                            Routes.dashboardRoute <|
                                model.highDensity
                   , FetchAllTeams
                   , FetchAllResources
                   , FetchAllJobs
                   , FetchAllPipelines
                   , DeleteCachedPipelines
                   , DeleteCachedJobs
                   , DeleteCachedTeams
                   ]
            )

        PipelineToggled _ (Ok ()) ->
            ( model, effects ++ [ FetchAllPipelines ] )

        VisibilityChanged Hide pipelineId (Ok ()) ->
            ( updatePipeline
                (\p -> { p | public = False, isVisibilityLoading = False })
                pipelineId
                model
            , effects
            )

        VisibilityChanged Hide pipelineId (Err _) ->
            ( updatePipeline
                (\p -> { p | public = True, isVisibilityLoading = False })
                pipelineId
                model
            , effects
            )

        VisibilityChanged Expose pipelineId (Ok ()) ->
            ( updatePipeline
                (\p -> { p | public = True, isVisibilityLoading = False })
                pipelineId
                model
            , effects
            )

        VisibilityChanged Expose pipelineId (Err _) ->
            ( updatePipeline
                (\p -> { p | public = False, isVisibilityLoading = False })
                pipelineId
                model
            , effects
            )

        GotViewport Dashboard (Ok viewport) ->
            ( { model
                | viewportWidth = viewport.viewport.width
                , viewportHeight = viewport.viewport.height
                , scrollTop = viewport.viewport.y
              }
            , effects
            )

        _ ->
            ( model, effects )
    )
        |> RequestBuffer.handleCallback callback buffers


updatePipeline :
    (Pipeline -> Pipeline)
    -> Concourse.PipelineIdentifier
    -> Model
    -> Model
updatePipeline updater pipelineId model =
    { model
        | pipelines =
            model.pipelines
                |> FetchResult.map
                    (List.Extra.updateIf
                        (\p ->
                            p.teamName == pipelineId.teamName && p.name == pipelineId.pipelineName
                        )
                        updater
                    )
    }


handleDelivery : Delivery -> ET Model
handleDelivery delivery =
    SearchBar.handleDelivery delivery
        >> Footer.handleDelivery delivery
        >> RequestBuffer.handleDelivery delivery buffers
        >> handleDeliveryBody delivery


handleDeliveryBody : Delivery -> ET Model
handleDeliveryBody delivery ( model, effects ) =
    case delivery of
        ClockTicked OneSecond time ->
            ( { model | now = Just time, effectsToRetry = [] }, model.effectsToRetry )

        WindowResized _ _ ->
            ( model, effects ++ [ GetViewportOf Dashboard ] )

        SideBarStateReceived _ ->
            ( model, effects ++ [ GetViewportOf Dashboard ] )

        CachedPipelinesReceived (Ok pipelines) ->
            let
                newPipelines =
                    pipelines
                        |> List.map (toDashboardPipeline True (model.jobsError == Just Disabled))
                        |> Cached
            in
            if newPipelines |> pipelinesChangedFrom model.pipelines then
                ( { model | pipelines = newPipelines }, effects )

            else
                ( model, effects )

        CachedJobsReceived (Ok jobs) ->
            let
                newJobs =
                    jobs
                        |> List.map
                            (\job ->
                                ( ( job.teamName
                                  , job.pipelineName
                                  , job.name
                                  )
                                , job
                                )
                            )
                        |> Dict.fromList
                        |> Cached

                mapToJobIds jobsResult =
                    jobsResult
                        |> FetchResult.map (Dict.toList >> List.map Tuple.first)
            in
            if mapToJobIds newJobs |> changedFrom (mapToJobIds model.jobs) then
                ( { model | jobs = newJobs } |> precomputeJobMetadata
                , effects
                )

            else
                ( model, effects )

        CachedTeamsReceived (Ok teams) ->
            let
                newTeams =
                    Cached teams
            in
            if newTeams |> changedFrom model.teams then
                ( { model | teams = newTeams }, effects )

            else
                ( model, effects )

        _ ->
            ( model, effects )


toDashboardPipeline : Bool -> Bool -> Concourse.Pipeline -> Pipeline
toDashboardPipeline isStale jobsDisabled p =
    { id = p.id
    , name = p.name
    , teamName = p.teamName
    , public = p.public
    , isToggleLoading = False
    , isVisibilityLoading = False
    , paused = p.paused
    , archived = p.archived
    , stale = isStale
    , jobsDisabled = jobsDisabled
    }


toConcoursePipeline : Pipeline -> Concourse.Pipeline
toConcoursePipeline p =
    { id = p.id
    , name = p.name
    , teamName = p.teamName
    , public = p.public
    , paused = p.paused
    , archived = p.archived
    , groups = []
    }


pipelinesChangedFrom : FetchResult (List Pipeline) -> FetchResult (List Pipeline) -> Bool
pipelinesChangedFrom ps qs =
    let
        project =
            FetchResult.map <| List.map (\x -> { x | stale = True })
    in
    changedFrom (project ps) (project qs)


groupBy : (a -> comparable) -> List a -> Dict comparable (List a)
groupBy keyfn list =
    -- From https://github.com/elm-community/dict-extra/blob/2.3.0/src/Dict/Extra.elm
    List.foldr
        (\x acc ->
            Dict.update (keyfn x) (Maybe.map ((::) x) >> Maybe.withDefault [ x ] >> Just) acc
        )
        Dict.empty
        list


precomputeJobMetadata : Model -> Model
precomputeJobMetadata model =
    let
        allJobs =
            model.jobs
                |> FetchResult.withDefault Dict.empty
                |> Dict.values

        pipelineJobs =
            allJobs |> groupBy (\j -> ( j.teamName, j.pipelineName ))

        jobToId job =
            { teamName = job.teamName
            , pipelineName = job.pipelineName
            , jobName = job.name
            }
    in
    { model
        | pipelineLayers =
            pipelineJobs
                |> Dict.map
                    (\_ jobs ->
                        jobs
                            |> DashboardPreview.groupByRank
                            |> List.map (List.map jobToId)
                    )
        , pipelineJobs =
            pipelineJobs
                |> Dict.map (\_ jobs -> jobs |> List.map jobToId)
    }


update : Session -> Message -> ET Model
update session msg =
    SearchBar.update session msg >> updateBody msg


updateBody : Message -> ET Model
updateBody msg ( model, effects ) =
    case msg of
        DragStart teamName index ->
            ( { model | dragState = Models.Dragging teamName index }, effects )

        DragOver _ index ->
            ( { model | dropState = Models.Dropping index }, effects )

        TooltipHd pipelineName teamName ->
            ( model, effects ++ [ ShowTooltipHd ( pipelineName, teamName ) ] )

        Tooltip pipelineName teamName ->
            ( model, effects ++ [ ShowTooltip ( pipelineName, teamName ) ] )

        DragEnd ->
            case model.dragState of
                Dragging teamName dragIdx ->
                    let
                        teamStartIndex =
                            model.pipelines
                                |> FetchResult.withDefault []
                                |> List.Extra.findIndex (\p -> p.teamName == teamName)

                        pipelines =
                            case teamStartIndex of
                                Just teamStartIdx ->
                                    model.pipelines
                                        |> FetchResult.withDefault []
                                        |> Drag.drag
                                            (teamStartIdx + dragIdx)
                                            (teamStartIdx
                                                + (case model.dropState of
                                                    Dropping dropIdx ->
                                                        dropIdx

                                                    _ ->
                                                        dragIdx + 1
                                                  )
                                            )

                                _ ->
                                    model.pipelines |> FetchResult.withDefault []
                    in
                    ( { model
                        | pipelines = Fetched pipelines
                        , dragState = NotDragging
                        , dropState = DroppingWhileApiRequestInFlight teamName
                      }
                    , effects
                        ++ [ pipelines
                                |> List.filter (.teamName >> (==) teamName)
                                |> List.map .name
                                |> SendOrderPipelinesRequest teamName
                           , pipelines
                                |> List.map toConcoursePipeline
                                |> SaveCachedPipelines
                           ]
                    )

                _ ->
                    ( model, effects )

        Hover (Just domID) ->
            ( model, effects ++ [ GetViewportOf domID ] )

        Click LogoutButton ->
            ( { model
                | teams = None
                , pipelines = None
                , jobs = None
              }
            , effects
            )

        Click (PipelineButton pipelineId) ->
            let
                isPaused =
                    model.pipelines
                        |> FetchResult.withDefault []
                        |> List.Extra.find
                            (\p -> p.teamName == pipelineId.teamName && p.name == pipelineId.pipelineName)
                        |> Maybe.map .paused
            in
            case isPaused of
                Just ip ->
                    ( updatePipeline
                        (\p -> { p | isToggleLoading = True })
                        pipelineId
                        model
                    , effects
                        ++ [ SendTogglePipelineRequest pipelineId ip ]
                    )

                Nothing ->
                    ( model, effects )

        Click (VisibilityButton pipelineId) ->
            let
                isPublic =
                    model.pipelines
                        |> FetchResult.withDefault []
                        |> List.Extra.find
                            (\p -> p.teamName == pipelineId.teamName && p.name == pipelineId.pipelineName)
                        |> Maybe.map .public
            in
            case isPublic of
                Just public ->
                    ( updatePipeline
                        (\p -> { p | isVisibilityLoading = True })
                        pipelineId
                        model
                    , effects
                        ++ [ if public then
                                ChangeVisibility Hide pipelineId

                             else
                                ChangeVisibility Expose pipelineId
                           ]
                    )

                Nothing ->
                    ( model, effects )

        Click HamburgerMenu ->
            ( model, effects ++ [ GetViewportOf Dashboard ] )

        Scrolled scrollState ->
            ( { model | scrollTop = scrollState.scrollTop }, effects )

        _ ->
            ( model, effects )


subscriptions : List Subscription
subscriptions =
    [ OnClockTick OneSecond
    , OnClockTick FiveSeconds
    , OnMouse
    , OnKeyDown
    , OnKeyUp
    , OnWindowResize
    , OnCachedJobsReceived
    , OnCachedPipelinesReceived
    , OnCachedTeamsReceived
    ]


documentTitle : String
documentTitle =
    "Dashboard"


view : Session -> Model -> Html Message
view session model =
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ topBar session model
        , Html.div
            [ id "page-below-top-bar"
            , style "padding-top" "54px"
            , style "box-sizing" "border-box"
            , style "display" "flex"
            , style "height" "100%"
            , style "padding-bottom" <|
                if model.showHelp || model.hideFooter then
                    "0"

                else
                    "50px"
            ]
          <|
            [ SideBar.view session Nothing
            , dashboardView session model
            ]
        , Footer.view session model
        ]


tooltip : { a | pipelines : FetchResult (List Pipeline) } -> { b | hovered : HoverState.HoverState } -> Maybe Tooltip.Tooltip
tooltip model { hovered } =
    case hovered of
        HoverState.Tooltip (Message.PipelineStatusIcon _) _ ->
            Just
                { body =
                    Html.div
                        Styles.jobsDisabledTooltip
                        [ Html.text "automatic job monitoring disabled" ]
                , attachPosition = { direction = Tooltip.Top, alignment = Tooltip.Start }
                , arrow = Nothing
                }

        HoverState.Tooltip (Message.VisibilityButton { teamName, pipelineName }) _ ->
            model.pipelines
                |> FetchResult.withDefault []
                |> List.Extra.find
                    (\p ->
                        p.teamName == teamName && p.name == pipelineName
                    )
                |> Maybe.map
                    (\p ->
                        { body =
                            Html.div
                                Styles.visibilityTooltip
                                [ Html.text <|
                                    if p.public then
                                        "hide pipeline"

                                    else
                                        "expose pipeline"
                                ]
                        , attachPosition =
                            { direction = Tooltip.Top
                            , alignment = Tooltip.End
                            }
                        , arrow = Nothing
                        }
                    )

        _ ->
            Nothing


topBar : Session -> Model -> Html Message
topBar session model =
    Html.div
        (id "top-bar-app" :: Views.Styles.topBar False)
    <|
        [ Html.div [ style "display" "flex", style "align-items" "center" ]
            [ SideBar.hamburgerMenu session
            , Html.a (href "/" :: Views.Styles.concourseLogo) []
            , clusterNameView session
            ]
        ]
            ++ (let
                    isDropDownHidden =
                        model.dropdown == Hidden

                    isMobile =
                        session.screenSize == ScreenSize.Mobile
                in
                if
                    not model.highDensity
                        && isMobile
                        && (not isDropDownHidden || model.query /= "")
                then
                    [ SearchBar.view session model ]

                else if not model.highDensity then
                    [ SearchBar.view session model
                    , Login.view session.userState model False
                    ]

                else
                    [ Login.view session.userState model False ]
               )


clusterNameView : Session -> Html Message
clusterNameView session =
    Html.div
        Styles.clusterName
        [ Html.text session.clusterName ]


showTurbulence :
    { a
        | jobsError : Maybe FetchError
        , teamsError : Maybe FetchError
        , resourcesError : Maybe FetchError
        , pipelinesError : Maybe FetchError
    }
    -> Bool
showTurbulence model =
    (model.jobsError == Just Failed)
        || (model.teamsError == Just Failed)
        || (model.resourcesError == Just Failed)
        || (model.pipelinesError == Just Failed)


dashboardView :
    { a
        | hovered : HoverState.HoverState
        , screenSize : ScreenSize
        , userState : UserState.UserState
        , turbulenceImgSrc : String
        , pipelineRunningKeyframes : String
    }
    -> Model
    -> Html Message
dashboardView session model =
    if showTurbulence model then
        turbulenceView session.turbulenceImgSrc

    else
        Html.div
            (class (.pageBodyClass Message.Effects.stickyHeaderConfig)
                :: id (toHtmlID Dashboard)
                :: onScroll Scrolled
                :: Styles.content model.highDensity
            )
            (case FetchResult.value model.pipelines of
                Nothing ->
                    [ loadingView ]

                Just [] ->
                    welcomeCard session :: pipelinesView session model

                Just _ ->
                    Html.text "" :: pipelinesView session model
            )


loadingView : Html Message
loadingView =
    Html.div
        (class "loading" :: Styles.loadingView)
        [ Spinner.spinner { sizePx = 36, margin = "0" } ]


welcomeCard :
    { a | hovered : HoverState.HoverState, userState : UserState.UserState }
    -> Html Message
welcomeCard session =
    let
        cliIcon : HoverState.HoverState -> Cli.Cli -> Html Message
        cliIcon hoverable cli =
            Html.a
                ([ href <| Cli.downloadUrl cli
                 , attribute "aria-label" <| Cli.label cli
                 , id <| "top-cli-" ++ Cli.id cli
                 , onMouseEnter <| Hover <| Just <| Message.WelcomeCardCliIcon cli
                 , onMouseLeave <| Hover Nothing
                 , download ""
                 ]
                    ++ Styles.topCliIcon
                        { hovered =
                            HoverState.isHovered
                                (Message.WelcomeCardCliIcon cli)
                                hoverable
                        , cli = cli
                        }
                )
                []
    in
    Html.div
        (id "welcome-card" :: Styles.welcomeCard)
        [ Html.div
            Styles.welcomeCardTitle
            [ Html.text Text.welcome ]
        , Html.div
            Styles.welcomeCardBody
          <|
            [ Html.div
                [ style "display" "flex"
                , style "align-items" "center"
                ]
              <|
                [ Html.div
                    [ style "margin-right" "10px" ]
                    [ Html.text Text.cliInstructions ]
                ]
                    ++ List.map (cliIcon session.hovered) Cli.clis
            , Html.div
                []
                [ Html.text Text.setPipelineInstructions ]
            ]
                ++ loginInstruction session.userState
        , Html.pre
            Styles.asciiArt
            [ Html.text Text.asciiArt ]
        ]


loginInstruction : UserState.UserState -> List (Html Message)
loginInstruction userState =
    case userState of
        UserState.UserStateLoggedIn _ ->
            []

        _ ->
            [ Html.div
                [ id "login-instruction"
                , style "line-height" "42px"
                ]
                [ Html.text "login "
                , Html.a
                    [ href "/login"
                    , style "text-decoration" "underline"
                    ]
                    [ Html.text "here" ]
                ]
            ]


noResultsView : String -> Html Message
noResultsView query =
    let
        boldedQuery =
            Html.span [ class "monospace-bold" ] [ Html.text query ]
    in
    Html.div
        (class "no-results" :: Styles.noResults)
        [ Html.text "No results for "
        , boldedQuery
        , Html.text " matched your search."
        ]


turbulenceView : String -> Html Message
turbulenceView path =
    Html.div
        [ class "error-message" ]
        [ Html.div [ class "message" ]
            [ Html.img [ src path, class "seatbelt" ] []
            , Html.p [] [ Html.text "experiencing turbulence" ]
            , Html.p [ class "explanation" ] []
            ]
        ]


pipelinesView :
    { a
        | userState : UserState.UserState
        , hovered : HoverState.HoverState
        , pipelineRunningKeyframes : String
    }
    ->
        { b
            | teams : FetchResult (List Concourse.Team)
            , query : String
            , highDensity : Bool
            , pipelinesWithResourceErrors : Dict ( String, String ) Bool
            , pipelineLayers : Dict ( String, String ) (List (List Concourse.JobIdentifier))
            , pipelines : FetchResult (List Pipeline)
            , jobs : FetchResult (Dict ( String, String, String ) Concourse.Job)
            , dragState : DragState
            , dropState : DropState
            , now : Maybe Time.Posix
            , viewportWidth : Float
            , viewportHeight : Float
            , scrollTop : Float
            , pipelineJobs : Dict ( String, String ) (List Concourse.JobIdentifier)
        }
    -> List (Html Message)
pipelinesView session params =
    let
        pipelines =
            params.pipelines
                |> FetchResult.withDefault []
                |> List.filter (not << .archived)

        jobs =
            params.jobs
                |> FetchResult.withDefault Dict.empty

        teams =
            params.teams
                |> FetchResult.withDefault []

        filteredGroups =
            Filter.filterGroups
                { pipelineJobs = params.pipelineJobs
                , jobs = jobs
                , query = params.query
                , teams = teams
                , pipelines = pipelines
                }
                |> List.sortWith (Group.ordering session)

        groupViews =
            filteredGroups
                |> (if params.highDensity then
                        List.concatMap
                            (Group.hdView
                                { pipelineRunningKeyframes = session.pipelineRunningKeyframes
                                , pipelinesWithResourceErrors = params.pipelinesWithResourceErrors
                                , pipelineJobs = params.pipelineJobs
                                , jobs = jobs
                                }
                                session
                            )

                    else
                        List.foldl
                            (\g ( htmlList, totalOffset ) ->
                                let
                                    layout =
                                        PipelineGrid.computeLayout
                                            { dragState = params.dragState
                                            , dropState = params.dropState
                                            , pipelineLayers = params.pipelineLayers
                                            , viewportWidth = params.viewportWidth
                                            , viewportHeight = params.viewportHeight
                                            , scrollTop = params.scrollTop - totalOffset
                                            }
                                            g
                                in
                                Group.view
                                    session
                                    { dragState = params.dragState
                                    , dropState = params.dropState
                                    , now = params.now
                                    , hovered = session.hovered
                                    , pipelineRunningKeyframes = session.pipelineRunningKeyframes
                                    , pipelinesWithResourceErrors = params.pipelinesWithResourceErrors
                                    , pipelineLayers = params.pipelineLayers
                                    , query = params.query
                                    , pipelineCards = layout.pipelineCards
                                    , dropAreas = layout.dropAreas
                                    , groupCardsHeight = layout.height
                                    , pipelineJobs = params.pipelineJobs
                                    , jobs = jobs
                                    }
                                    g
                                    |> (\html ->
                                            ( html :: htmlList
                                            , totalOffset
                                                + layout.height
                                                + PipelineGridConstants.headerHeight
                                                + PipelineGridConstants.padding
                                            )
                                       )
                            )
                            ( [], 0 )
                            >> Tuple.first
                            >> List.reverse
                   )
    in
    if
        (params.pipelines /= None)
            && List.isEmpty groupViews
            && not (String.isEmpty params.query)
    then
        [ noResultsView params.query ]

    else
        groupViews
