module Dashboard.Dashboard exposing
    ( changeRoute
    , documentTitle
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , tooltip
    , update
    , view
    )

import Application.Models exposing (Session)
import Colors
import Concourse exposing (hyphenNotation)
import Concourse.BuildStatus
import Concourse.Cli as Cli
import Dashboard.DashboardPreview as DashboardPreview
import Dashboard.Drag as Drag
import Dashboard.Filter as Filter
import Dashboard.Footer as Footer
import Dashboard.Grid as Grid
import Dashboard.Grid.Constants as GridConstants
import Dashboard.Group as Group
import Dashboard.Group.Models exposing (Card(..), Pipeline, cardName)
import Dashboard.Models as Models
    exposing
        ( DragState(..)
        , DropState(..)
        , Dropdown(..)
        , FetchError(..)
        , Model
        )
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
import Message.ScrollDirection exposing (ScrollDirection(..))
import Message.Subscription
    exposing
        ( Delivery(..)
        , Interval(..)
        , Subscription(..)
        )
import Ordering
import Routes
import ScreenSize exposing (ScreenSize(..))
import Set
import SideBar.SideBar as SideBar exposing (byDatabaseId, byPipelineId, lookupPipeline)
import StrictEvents exposing (onScroll)
import Tooltip
import UserState
import Views.Spinner as Spinner
import Views.Styles
import Views.Toggle as Toggle
import Views.TopBar as TopBar


type alias Flags =
    { searchType : Routes.SearchType
    , dashboardView : Routes.DashboardView
    }


init : Flags -> ( Model, List Effect )
init f =
    ( { now = Nothing
      , hideFooter = False
      , hideFooterCounter = 0
      , showHelp = False
      , highDensity = f.searchType == Routes.HighDensity
      , query = Routes.extractQuery f.searchType
      , instanceGroup = Routes.extractInstanceGroup f.searchType
      , dashboardView = f.dashboardView
      , pipelinesWithResourceErrors = Set.empty
      , jobs = None
      , pipelines = Nothing
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


changeRoute : Flags -> ET Model
changeRoute f ( model, effects ) =
    ( { model
        | highDensity = f.searchType == Routes.HighDensity
        , dashboardView = f.dashboardView

        -- I think we're deliberately not extracting the query from searchType
        -- so that switching to the HD view carries the search query over?
        -- Eventually, the HD view should have search functionality, though
        , instanceGroup = Routes.extractInstanceGroup f.searchType
      }
    , effects
        ++ (if model.instanceGroup /= Routes.extractInstanceGroup f.searchType then
                [ Scroll ToTop <| toHtmlID Dashboard ]

            else
                []
           )
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

                newJobs : FetchResult (Dict ( Int, String ) Concourse.Job)
                newJobs =
                    allJobsInEntireCluster
                        |> List.map
                            (\job ->
                                ( ( job.pipelineId
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
                                        |> Maybe.map
                                            (Dict.map
                                                (\_ l ->
                                                    List.map
                                                        (\p ->
                                                            { p | jobsDisabled = True }
                                                        )
                                                        l
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
            let
                failingToCheck { build } =
                    case build of
                        Nothing ->
                            False

                        Just { status } ->
                            Concourse.BuildStatus.isBad status
            in
            ( { model
                | pipelinesWithResourceErrors =
                    resources
                        |> List.filter failingToCheck
                        |> List.map (\r -> r.pipelineId)
                        |> Set.fromList
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
                        |> groupBy .teamName
                        |> Just
            in
            ( { model
                | pipelines = newPipelines
                , pipelinesError = Nothing
              }
            , effects
                ++ GetViewportOf Dashboard
                :: (if List.isEmpty allPipelinesInEntireCluster then
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
                            Routes.Dashboard
                                { searchType =
                                    if model.highDensity then
                                        Routes.HighDensity

                                    else
                                        Routes.Normal model.query Nothing
                                , dashboardView = model.dashboardView
                                }
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
                |> Maybe.map
                    (Dict.update pipelineId.teamName
                        (Maybe.map
                            (List.Extra.updateIf
                                (\p ->
                                    (p.name == pipelineId.pipelineName)
                                        && (p.instanceVars == pipelineId.pipelineInstanceVars)
                                )
                                updater
                            )
                        )
                    )
    }


handleDelivery : Session -> Delivery -> ET Model
handleDelivery session delivery =
    SearchBar.handleDelivery session delivery
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
            if model.pipelines == Nothing then
                ( { model
                    | pipelines =
                        pipelines
                            |> List.map
                                (toDashboardPipeline
                                    True
                                    (model.jobsError == Just Disabled)
                                )
                            |> groupBy .teamName
                            |> Just
                  }
                , effects
                )

            else
                ( model, effects )

        CachedJobsReceived (Ok jobs) ->
            let
                newJobs =
                    jobs
                        |> List.map
                            (\job ->
                                ( ( job.pipelineId
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
    , instanceVars = p.instanceVars
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
    , instanceVars = p.instanceVars
    , teamName = p.teamName
    , public = p.public
    , paused = p.paused
    , archived = p.archived
    , groups = []
    , backgroundImage = Maybe.Nothing
    }


pipelinesChangedFrom :
    Maybe (Dict String (List Pipeline))
    -> Maybe (Dict String (List Pipeline))
    -> Bool
pipelinesChangedFrom ps qs =
    let
        project =
            Maybe.map <|
                Dict.values
                    >> List.concat
                    >> List.map (\x -> { x | stale = True })
    in
    project ps /= project qs


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
            allJobs |> groupBy (\j -> j.pipelineId)
    in
    { model
        | pipelineLayers =
            pipelineJobs
                |> Dict.map
                    (\_ jobs ->
                        jobs
                            |> DashboardPreview.groupByRank
                            |> List.map (List.map .name)
                    )
        , pipelineJobs =
            pipelineJobs
                |> Dict.map (\_ jobs -> jobs |> List.map .name)
    }


update : Session -> Message -> ET Model
update session msg =
    SearchBar.update session msg >> updateBody session msg


updateBody : Session -> Message -> ET Model
updateBody session msg ( model, effects ) =
    case msg of
        DragStart teamName cardId ->
            ( { model | dragState = Models.Dragging teamName cardId }, effects )

        DragOver target ->
            ( { model | dropState = Models.Dropping target }, effects )

        DragEnd ->
            case ( model.dragState, model.dropState ) of
                ( Dragging teamName name, Dropping target ) ->
                    let
                        teamCards =
                            model.pipelines
                                |> Maybe.andThen (Dict.get teamName)
                                |> Maybe.withDefault []
                                |> groupCards
                                |> (\cards ->
                                        cards
                                            |> (case Drag.dragCardIndices name target cards of
                                                    Just ( from, to ) ->
                                                        Drag.drag from to

                                                    _ ->
                                                        identity
                                               )
                                   )

                        pipelines =
                            model.pipelines
                                |> Maybe.withDefault Dict.empty
                                |> Dict.update teamName
                                    (always <|
                                        Just <|
                                            List.concatMap
                                                (\card ->
                                                    case card of
                                                        PipelineCard p ->
                                                            [ p ]

                                                        InstanceGroupCard p ps ->
                                                            p :: ps
                                                )
                                                teamCards
                                    )
                    in
                    ( { model
                        | pipelines = Just pipelines
                        , dragState = NotDragging
                        , dropState = DroppingWhileApiRequestInFlight teamName
                      }
                    , effects
                        ++ [ teamCards
                                |> List.map cardName
                                |> SendOrderPipelinesRequest teamName
                           , pipelines
                                |> Dict.values
                                |> List.concat
                                |> List.map toConcoursePipeline
                                |> SaveCachedPipelines
                           ]
                    )

                _ ->
                    ( { model
                        | dragState = NotDragging
                        , dropState = NotDropping
                      }
                    , effects
                    )

        Click LogoutButton ->
            ( { model
                | teams = None
                , pipelines = Nothing
                , jobs = None
              }
            , effects
            )

        Click (PipelineCardPauseToggle _ pipelineId) ->
            let
                isPaused =
                    session
                        |> lookupPipeline (byPipelineId pipelineId)
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

        Click (VisibilityButton _ id) ->
            case lookupPipeline (byDatabaseId id) session of
                Just pipeline ->
                    let
                        pipelineId =
                            Concourse.toPipelineId pipeline
                    in
                    ( updatePipeline
                        (\p -> { p | isVisibilityLoading = True })
                        pipelineId
                        model
                    , effects
                        ++ [ if pipeline.public then
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
            [ SideBar.view session
                (model.instanceGroup
                    |> Maybe.map
                        (\{ name, teamName } ->
                            { teamName = teamName
                            , pipelineName = name
                            }
                        )
                )
            , dashboardView session model
            ]
        , Footer.view session model
        ]


tooltip : Session -> Maybe Tooltip.Tooltip
tooltip session =
    case session.hovered of
        HoverState.Tooltip (Message.PipelineStatusIcon _ _) _ ->
            Just
                { body =
                    Html.div
                        Styles.jobsDisabledTooltip
                        [ Html.text "automatic job monitoring disabled" ]
                , attachPosition = { direction = Tooltip.Top, alignment = Tooltip.Start }
                , arrow = Nothing
                }

        HoverState.Tooltip (Message.VisibilityButton _ id) _ ->
            session
                |> lookupPipeline (byDatabaseId id)
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

        HoverState.Tooltip (Message.JobPreview _ _ jobName) _ ->
            Just
                { body =
                    Html.div
                        Styles.jobPreviewTooltip
                        [ Html.text jobName ]
                , attachPosition = { direction = Tooltip.Right 0, alignment = Tooltip.Middle 30 }
                , arrow = Just { size = 15, color = "#000" }
                }

        HoverState.Tooltip (Message.PipelinePreview _ id) _ ->
            session
                |> lookupPipeline (byDatabaseId id)
                |> Maybe.map
                    (\p ->
                        { body =
                            Html.div
                                Styles.pipelinePreviewTooltip
                                [ Html.text <| hyphenNotation p.instanceVars ]
                        , attachPosition =
                            { direction = Tooltip.Right 0
                            , alignment = Tooltip.Middle 30
                            }
                        , arrow = Just { size = 15, color = "#000" }
                        }
                    )

        HoverState.Tooltip (Message.PipelineCardName _ id) _ ->
            session
                |> lookupPipeline (byDatabaseId id)
                |> Maybe.map
                    (\p ->
                        { body =
                            Html.div
                                Styles.cardTooltip
                                [ Html.text p.name ]
                        , attachPosition =
                            { direction = Tooltip.Right 0
                            , alignment = Tooltip.Middle 30
                            }
                        , arrow = Just { size = 15, color = "#000" }
                        }
                    )

        HoverState.Tooltip (Message.PipelineCardNameHD id) _ ->
            session
                |> lookupPipeline (byDatabaseId id)
                |> Maybe.map
                    (\p ->
                        { body =
                            Html.div
                                Styles.cardTooltip
                                [ Html.text p.name ]
                        , attachPosition =
                            { direction = Tooltip.Right 0
                            , alignment = Tooltip.Middle 30
                            }
                        , arrow = Just { size = 15, color = "#000" }
                        }
                    )

        HoverState.Tooltip (Message.InstanceGroupCardName _ _ groupName) _ ->
            Just
                { body =
                    Html.div
                        Styles.cardTooltip
                        [ Html.text groupName ]
                , attachPosition = { direction = Tooltip.Right 0, alignment = Tooltip.Middle 30 }
                , arrow = Just { size = 15, color = "#000" }
                }

        HoverState.Tooltip (Message.InstanceGroupCardNameHD _ groupName) _ ->
            Just
                { body =
                    Html.div
                        Styles.cardTooltip
                        [ Html.text groupName ]
                , attachPosition = { direction = Tooltip.Right 0, alignment = Tooltip.Middle 30 }
                , arrow = Just { size = 15, color = "#000" }
                }

        HoverState.Tooltip (Message.PipelineCardInstanceVar _ _ key value) _ ->
            Just
                { body =
                    Html.div
                        Styles.cardTooltip
                        [ Html.text <| key ++ ": " ++ value ]
                , attachPosition = { direction = Tooltip.Right 0, alignment = Tooltip.Middle 30 }
                , arrow = Just { size = 15, color = "#000" }
                }

        _ ->
            Nothing


topBar : Session -> Model -> Html Message
topBar session model =
    Html.div
        (id "top-bar-app" :: Views.Styles.topBar False)
    <|
        [ Html.div [ style "display" "flex", style "align-items" "center" ]
            [ SideBar.hamburgerMenu session
            , TopBar.concourseLogo
            , TopBar.breadcrumbs session session.route
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
                    [ topBarContent [ SearchBar.view session model ]
                    , showArchivedToggleView model
                    , Login.view session.userState model
                    ]

                else
                    [ topBarContent []
                    , showArchivedToggleView model
                    , Login.view session.userState model
                    ]
               )


topBarContent : List (Html Message) -> Html Message
topBarContent content =
    Html.div
        (id "top-bar-content" :: Styles.topBarContent)
        content


showArchivedToggleView : Model -> Html Message
showArchivedToggleView model =
    let
        noPipelines =
            model.pipelines
                |> Maybe.withDefault Dict.empty
                |> Dict.values
                |> List.all List.isEmpty

        on =
            model.dashboardView == Routes.ViewAllPipelines
    in
    if noPipelines then
        Html.text ""

    else
        Toggle.toggleSwitch
            { ariaLabel = "Toggle whether archived pipelines are displayed"
            , hrefRoute =
                Routes.Dashboard
                    { searchType =
                        if model.highDensity then
                            Routes.HighDensity

                        else
                            Routes.Normal model.query model.instanceGroup
                    , dashboardView =
                        if on then
                            Routes.ViewNonArchivedPipelines

                        else
                            Routes.ViewAllPipelines
                    }
            , text = "show archived"
            , textDirection = Toggle.Left
            , on = on
            , styles = Styles.showArchivedToggle
            }


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


dashboardView : Session -> Model -> Html Message
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
            (case model.pipelines of
                Nothing ->
                    [ loadingView ]

                Just pipelines ->
                    if pipelines |> Dict.values |> List.all List.isEmpty then
                        welcomeCard session :: dashboardCardsView session model

                    else
                        Html.text "" :: dashboardCardsView session model
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
                    , style "color" Colors.welcomeCardText
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


dashboardCardsView : Session -> Model -> List (Html Message)
dashboardCardsView session model =
    case model.instanceGroup of
        Nothing ->
            regularCardsView session model

        Just ig ->
            instanceGroupCardsView session model ig


regularCardsView : Session -> Model -> List (Html Message)
regularCardsView session params =
    let
        pipelines =
            params.pipelines
                |> Maybe.withDefault Dict.empty

        jobs =
            params.jobs
                |> FetchResult.withDefault Dict.empty

        teams =
            params.teams
                |> FetchResult.withDefault []

        filteredPipelinesByTeam =
            Filter.filterTeams session params
                |> Dict.toList
                |> List.sortWith (Ordering.byFieldWith (Group.ordering session) Tuple.first)

        teamCards =
            filteredPipelinesByTeam
                |> List.map
                    (\( team, teamPipelines ) ->
                        ( team
                        , groupCards teamPipelines
                        )
                    )
    in
    cardsView session params teamCards


groupCards : List Pipeline -> List Card
groupCards =
    List.Extra.gatherEqualsBy .name
        >> List.map
            (\( p, ps ) ->
                if List.isEmpty ps && Dict.isEmpty p.instanceVars then
                    PipelineCard p

                else
                    InstanceGroupCard p ps
            )


instanceGroupCardsView : Session -> Model -> Concourse.InstanceGroupIdentifier -> List (Html Message)
instanceGroupCardsView session model { teamName, name } =
    let
        pipelines =
            model.pipelines
                |> Maybe.withDefault Dict.empty

        jobs =
            model.jobs
                |> FetchResult.withDefault Dict.empty

        pipelineInstances =
            pipelines
                |> Dict.get teamName
                |> Maybe.map (List.filter (.name >> (==) name))
                |> Maybe.withDefault []

        filteredPipelines =
            Filter.filterTeams session
                { model
                    | pipelines =
                        Just <|
                            Dict.fromList [ ( teamName, pipelineInstances ) ]
                }
                |> Dict.toList
                |> List.head
                |> Maybe.map Tuple.second
                |> Maybe.withDefault []
    in
    cardsView session
        model
        [ ( teamName ++ " / " ++ name
          , filteredPipelines |> List.map PipelineCard
          )
        ]


cardsView : Session -> Model -> List ( String, List Card ) -> List (Html Message)
cardsView session params teamCards =
    let
        jobs =
            params.jobs
                |> FetchResult.withDefault Dict.empty

        ( headerView, offsetHeight ) =
            if params.highDensity then
                ( [], 0 )

            else
                let
                    favoritedCards =
                        teamCards
                            |> List.concatMap Tuple.second
                            |> List.filterMap
                                (\c ->
                                    let
                                        isFavorited p =
                                            Set.member p.id session.favoritedPipelines
                                    in
                                    case c of
                                        PipelineCard p ->
                                            if isFavorited p then
                                                Just <| PipelineCard p

                                            else
                                                Nothing

                                        InstanceGroupCard p ps ->
                                            case List.filter isFavorited (p :: ps) of
                                                x :: xs ->
                                                    Just <| InstanceGroupCard x xs

                                                [] ->
                                                    Nothing
                                )

                    allPipelinesHeader =
                        Html.div Styles.pipelineSectionHeader [ Html.text "all pipelines" ]
                in
                if List.isEmpty teamCards then
                    ( [], 0 )

                else if List.isEmpty favoritedCards then
                    ( [ allPipelinesHeader ], GridConstants.sectionHeaderHeight )

                else
                    let
                        offset =
                            GridConstants.sectionHeaderHeight

                        layout =
                            Grid.computeFavoritesLayout
                                { pipelineLayers = params.pipelineLayers
                                , viewportWidth = params.viewportWidth
                                , viewportHeight = params.viewportHeight
                                , scrollTop = params.scrollTop - offset
                                , isInstanceGroupView = params.instanceGroup /= Nothing
                                }
                                favoritedCards
                    in
                    [ Html.div Styles.pipelineSectionHeader [ Html.text "favorite pipelines" ]
                    , Group.viewFavoritePipelines
                        session
                        { dragState = NotDragging
                        , dropState = NotDropping
                        , now = params.now
                        , pipelinesWithResourceErrors = params.pipelinesWithResourceErrors
                        , pipelineLayers = params.pipelineLayers
                        , groupCardsHeight = layout.height
                        , pipelineJobs = params.pipelineJobs
                        , jobs = jobs
                        , dashboardView = params.dashboardView
                        , query = params.query
                        , inInstanceGroupView = params.instanceGroup /= Nothing
                        }
                        layout.headers
                        layout.cards
                    , Views.Styles.separator GridConstants.sectionSpacerHeight
                    , allPipelinesHeader
                    ]
                        |> (\html ->
                                ( html
                                , layout.height
                                    + (2 * GridConstants.sectionHeaderHeight)
                                    + GridConstants.sectionSpacerHeight
                                )
                           )

        groupViews =
            teamCards
                |> (if params.highDensity then
                        List.concatMap
                            (Group.hdView
                                { pipelinesWithResourceErrors = params.pipelinesWithResourceErrors
                                , pipelineJobs = params.pipelineJobs
                                , jobs = jobs
                                , dashboardView = params.dashboardView
                                , query = params.query
                                }
                                session
                            )

                    else
                        List.foldl
                            (\( teamName, cards ) ( htmlList, totalOffset ) ->
                                let
                                    startingOffset =
                                        totalOffset
                                            + GridConstants.groupHeaderHeight
                                            + GridConstants.padding

                                    layout =
                                        Grid.computeLayout
                                            { dragState = params.dragState
                                            , dropState = params.dropState
                                            , pipelineLayers = params.pipelineLayers
                                            , viewportWidth = params.viewportWidth
                                            , viewportHeight = params.viewportHeight
                                            , scrollTop = params.scrollTop - startingOffset
                                            }
                                            teamName
                                            cards
                                in
                                Group.view
                                    session
                                    { dragState = params.dragState
                                    , dropState = params.dropState
                                    , now = params.now
                                    , pipelinesWithResourceErrors = params.pipelinesWithResourceErrors
                                    , pipelineLayers = params.pipelineLayers
                                    , dropAreas = layout.dropAreas
                                    , groupCardsHeight = layout.height
                                    , pipelineJobs = params.pipelineJobs
                                    , jobs = jobs
                                    , dashboardView = params.dashboardView
                                    , query = params.query
                                    , inInstanceGroupView = params.instanceGroup /= Nothing
                                    }
                                    teamName
                                    layout.cards
                                    |> (\html ->
                                            ( html :: htmlList
                                            , startingOffset + layout.height
                                            )
                                       )
                            )
                            ( [], offsetHeight )
                            >> Tuple.first
                            >> List.reverse
                   )
    in
    if
        (params.pipelines /= Nothing)
            && List.isEmpty groupViews
            && not (String.isEmpty params.query)
    then
        [ noResultsView params.query ]

    else
        headerView ++ groupViews
