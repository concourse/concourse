module Dashboard.Dashboard exposing
    ( documentTitle
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
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
import Dashboard.Models as Models exposing (DragState(..), DropState(..), Dropdown(..), Model)
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
import List.Extra
import Login.Login as Login
import Message.Callback exposing (Callback(..), TooltipPolicy(..))
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
import UserState
import Views.Styles


init : Routes.SearchType -> ( Model, List Effect )
init searchType =
    ( { showTurbulence = False
      , now = Nothing
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
      , viewportWidth = 0
      , viewportHeight = 0
      , scrollTop = 0
      , pipelineJobs = Dict.empty
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
      , GetViewportOf Dashboard AlwaysShow
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
        (.dragState >> (/=) NotDragging)
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
            ( { model | showTurbulence = True }, effects )

        AllTeamsFetched (Ok teams) ->
            let
                newTeams =
                    Fetched teams
            in
            ( { model | teams = newTeams }
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
                    Fetched allJobsInEntireCluster

                newModel =
                    { model | jobs = newJobs }
            in
            if newJobs |> changedFrom model.jobs then
                ( newModel |> precomputeJobMetadata
                , effects
                    ++ [ allJobsInEntireCluster
                            |> List.map removeBuild
                            |> SaveCachedJobs
                       ]
                )

            else
                ( newModel, effects )

        AllJobsFetched (Err _) ->
            ( { model | showTurbulence = True }, effects )

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
              }
            , effects
            )

        AllResourcesFetched (Err _) ->
            ( { model | showTurbulence = True }, effects )

        AllPipelinesFetched (Ok allPipelinesInEntireCluster) ->
            let
                newPipelines =
                    allPipelinesInEntireCluster
                        |> List.map toDashboardPipeline
                        |> Fetched
            in
            ( { model | pipelines = newPipelines }
            , effects
                ++ (if List.isEmpty allPipelinesInEntireCluster then
                        [ ModifyUrl "/" ]

                    else
                        []
                   )
                ++ (if newPipelines |> changedFrom model.pipelines then
                        [ SaveCachedPipelines allPipelinesInEntireCluster ]

                    else
                        []
                   )
            )

        AllPipelinesFetched (Err _) ->
            ( { model | showTurbulence = True }, effects )

        PipelinesOrdered teamName _ ->
            ( model, effects ++ [ FetchPipelines teamName ] )

        PipelinesFetched (Ok _) ->
            ( { model | dropState = NotDropping }, effects )

        PipelinesFetched (Err _) ->
            ( { model | showTurbulence = True }, effects )

        LoggedOut (Ok ()) ->
            ( model
            , effects
                ++ [ NavigateTo <|
                        Routes.toString <|
                            Routes.dashboardRoute <|
                                model.highDensity
                   , FetchAllTeams
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

        GotViewport Dashboard _ (Ok viewport) ->
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
            ( { model | now = Just time }, effects )

        WindowResized _ _ ->
            ( model, effects ++ [ GetViewportOf Dashboard AlwaysShow ] )

        SideBarStateReceived _ ->
            ( model, effects ++ [ GetViewportOf Dashboard AlwaysShow ] )

        CachedPipelinesReceived (Ok pipelines) ->
            let
                newPipelines =
                    pipelines
                        |> List.map toDashboardPipeline
                        |> Cached
            in
            if newPipelines |> changedFrom model.pipelines then
                ( { model | pipelines = newPipelines }, effects )

            else
                ( model, effects )

        CachedJobsReceived (Ok jobs) ->
            let
                newJobs =
                    Cached jobs
            in
            if newJobs |> changedFrom model.jobs then
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


toDashboardPipeline : Concourse.Pipeline -> Pipeline
toDashboardPipeline p =
    { id = p.id
    , name = p.name
    , teamName = p.teamName
    , public = p.public
    , isToggleLoading = False
    , isVisibilityLoading = False
    , paused = p.paused
    }


precomputeJobMetadata : Model -> Model
precomputeJobMetadata model =
    let
        allJobs =
            model.jobs |> FetchResult.withDefault []

        pipelinesWithJobs =
            allJobs
                |> List.map (\j -> ( j.teamName, j.pipelineName ))
                |> List.Extra.unique
                |> List.map
                    (\( teamName, pipelineName ) ->
                        allJobs
                            |> List.filter (\j -> j.teamName == teamName && j.pipelineName == pipelineName)
                            |> Tuple.pair ( teamName, pipelineName )
                    )
    in
    { model
        | pipelineLayers =
            pipelinesWithJobs
                |> List.map (\( key, jobs ) -> ( key, DashboardPreview.groupByRank jobs ))
                |> Dict.fromList
        , pipelineJobs =
            pipelinesWithJobs
                |> Dict.fromList
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
                                        |> FetchResult.map
                                            (Drag.drag
                                                (teamStartIdx + dragIdx)
                                                (teamStartIdx
                                                    + (case model.dropState of
                                                        Dropping dropIdx ->
                                                            dropIdx

                                                        _ ->
                                                            dragIdx + 1
                                                      )
                                                )
                                            )

                                _ ->
                                    model.pipelines
                    in
                    ( { model
                        | pipelines = pipelines
                        , dragState = NotDragging
                        , dropState = DroppingWhileApiRequestInFlight teamName
                      }
                    , effects
                        ++ [ pipelines
                                |> FetchResult.withDefault []
                                |> List.filter (.teamName >> (==) teamName)
                                |> List.map .name
                                |> SendOrderPipelinesRequest teamName
                           ]
                    )

                _ ->
                    ( model, effects )

        Click LogoutButton ->
            ( { model | teams = None, pipelines = None, jobs = None }, effects )

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
            ( model, effects ++ [ GetViewportOf Dashboard AlwaysShow ] )

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
    if model.showTurbulence then
        turbulenceView session.turbulenceImgSrc

    else
        Html.div
            (class (.pageBodyClass Message.Effects.stickyHeaderConfig)
                :: id (toHtmlID Dashboard)
                :: onScroll Scrolled
                :: Styles.content model.highDensity
            )
            (welcomeCard session model :: pipelinesView session model)


welcomeCard :
    { a | hovered : HoverState.HoverState, userState : UserState.UserState }
    -> { b | pipelines : FetchResult (List Pipeline) }
    -> Html Message
welcomeCard session { pipelines } =
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

        noPipelines =
            pipelines
                |> FetchResult.withDefault []
                |> List.isEmpty
    in
    if noPipelines then
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

    else
        Html.text ""


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
            , pipelineLayers : Dict ( String, String ) (List (List Concourse.Job))
            , pipelines : FetchResult (List Pipeline)
            , jobs : FetchResult (List Concourse.Job)
            , dragState : DragState
            , dropState : DropState
            , now : Maybe Time.Posix
            , viewportWidth : Float
            , viewportHeight : Float
            , scrollTop : Float
            , pipelineJobs : Dict ( String, String ) (List Concourse.Job)
        }
    -> List (Html Message)
pipelinesView session params =
    let
        pipelines =
            params.pipelines
                |> FetchResult.withDefault []

        teams =
            params.teams
                |> FetchResult.withDefault []

        filteredGroups =
            Filter.filterGroups params.pipelineJobs params.query teams pipelines
                |> List.sortWith (Group.ordering session)

        isCached =
            case ( params.pipelines, params.jobs ) of
                ( Cached _, _ ) ->
                    True

                ( _, Cached _ ) ->
                    True

                _ ->
                    False

        groupViews =
            filteredGroups
                |> (if params.highDensity then
                        List.concatMap
                            (Group.hdView
                                { pipelineRunningKeyframes = session.pipelineRunningKeyframes
                                , pipelinesWithResourceErrors = params.pipelinesWithResourceErrors
                                , pipelineJobs = params.pipelineJobs
                                , isCached = isCached
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
                                    , isCached = isCached
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
    if List.isEmpty groupViews && not (String.isEmpty params.query) then
        [ noResultsView params.query ]

    else
        groupViews
