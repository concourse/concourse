module Benchmarks exposing (main)

import Ansi.Log
import Application.Models exposing (Session)
import Array
import Assets
import Benchmark
import Benchmark.Runner exposing (BenchmarkProgram, program)
import Build.Build as Build
import Build.Header.Models exposing (BuildPageType(..), CurrentOutput(..))
import Build.Models
import Build.Output.Models
import Build.Output.Output
import Build.StepTree.Models as STModels
import Build.Styles
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination exposing (Page)
import Dashboard.DashboardPreview as DP
import DateFormat
import Dict exposing (Dict)
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
        , tabindex
        , title
        )
import Html.Events exposing (onBlur, onFocus, onMouseEnter, onMouseLeave)
import Html.Lazy
import Keyboard
import Login.Login as Login
import Maybe.Extra
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import RemoteData exposing (WebData)
import Routes exposing (Highlight)
import ScreenSize
import Set
import SideBar.SideBar as SideBar
import StrictEvents exposing (onLeftClick, onScroll, onWheel)
import Time
import UserState
import Views.BuildDuration as BuildDuration
import Views.Icon as Icon
import Views.LoadingIndicator as LoadingIndicator
import Views.NotAuthorized as NotAuthorized
import Views.Spinner as Spinner
import Views.Styles
import Views.TopBar as TopBar


type alias Model =
    Login.Model
        { page : BuildPageType
        , now : Maybe Time.Posix
        , disableManualTrigger : Bool
        , history : List Concourse.Build
        , nextPage : Maybe Page
        , currentBuild : WebData CurrentBuild
        , autoScroll : Bool
        , previousKeyPress : Maybe Keyboard.KeyEvent
        , shiftDown : Bool
        , isTriggerBuildKeyDown : Bool
        , showHelp : Bool
        , highlight : Highlight
        , hoveredCounter : Int
        , fetchingHistory : Bool
        , scrolledToCurrentBuild : Bool
        , authorized : Bool
        }


type alias CurrentBuild =
    { build : Concourse.Build
    , prep : Maybe Concourse.BuildPrep
    , output : CurrentOutput
    }


main : BenchmarkProgram
main =
    program <|
        Benchmark.describe "benchmark suite"
            [ Benchmark.compare "DashboardPreview.view"
                "current"
                (\_ -> DP.view AllPipelinesSection HoverState.NoHover (DP.groupByRank sampleJobs))
                "old"
                (\_ -> dashboardPreviewView sampleJobs)
            , Benchmark.compare "Build.view"
                "current"
                (\_ -> Build.view sampleSession sampleModel)
                "old"
                (\_ -> buildView sampleSession sampleOldModel)
            ]


bodyId : String
bodyId =
    "build-body"


historyId : String
historyId =
    "builds"


buildView : Session -> Model -> Html Message
buildView session model =
    let
        route =
            case model.page of
                OneOffBuildPage buildId ->
                    Routes.OneOffBuild
                        { id = buildId
                        , highlight = model.highlight
                        }

                JobBuildPage buildId ->
                    Routes.Build
                        { id = buildId
                        , highlight = model.highlight
                        }
    in
    Html.div
        (id "page-including-top-bar" :: Views.Styles.pageIncludingTopBar)
        [ Html.div
            (id "top-bar-app" :: Views.Styles.topBar False)
            [ SideBar.hamburgerMenu session
            , TopBar.concourseLogo
            , breadcrumbs model
            , Login.view session.userState model
            ]
        , Html.div
            (id "page-below-top-bar" :: Views.Styles.pageBelowTopBar route)
            [ SideBar.view session
                (currentJob model
                    |> Maybe.map
                        (\j ->
                            { pipelineName = j.pipelineName
                            , teamName = j.teamName
                            }
                        )
                )
            , viewBuildPage session model
            ]
        ]


viewBuildPage : Session -> Model -> Html Message
viewBuildPage session model =
    case model.currentBuild |> RemoteData.toMaybe of
        Just currentBuild ->
            Html.div
                [ class "with-fixed-header"
                , attribute "data-build-name" currentBuild.build.name
                , style "flex-grow" "1"
                , style "display" "flex"
                , style "flex-direction" "column"
                , style "overflow" "hidden"
                ]
                [ viewBuildHeader session model currentBuild.build
                , body
                    session
                    { currentBuild = currentBuild
                    , authorized = model.authorized
                    , showHelp = model.showHelp
                    }
                ]

        _ ->
            LoadingIndicator.view


currentJob : Model -> Maybe Concourse.JobIdentifier
currentJob =
    .currentBuild
        >> RemoteData.toMaybe
        >> Maybe.map .build
        >> Maybe.andThen .job


breadcrumbs : Model -> Html Message
breadcrumbs model =
    case ( currentJob model, model.page ) of
        ( Just jobId, _ ) ->
            TopBar.breadcrumbs <|
                Routes.Job
                    { id = jobId
                    , page = Nothing
                    }

        ( _, JobBuildPage buildId ) ->
            TopBar.breadcrumbs <|
                Routes.Build
                    { id = buildId
                    , highlight = model.highlight
                    }

        _ ->
            Html.text ""


body :
    Session
    ->
        { currentBuild : CurrentBuild
        , authorized : Bool
        , showHelp : Bool
        }
    -> Html Message
body session { currentBuild, authorized, showHelp } =
    Html.div
        ([ class "scrollable-body build-body"
         , id bodyId
         , tabindex 0
         , onScroll Scrolled
         ]
            ++ Build.Styles.body
        )
    <|
        if authorized then
            [ viewBuildPrep currentBuild.prep
            , Html.Lazy.lazy2 viewBuildOutput session currentBuild.output
            , keyboardHelp showHelp
            ]
                ++ tombstone session.timeZone currentBuild

        else
            [ NotAuthorized.view ]


viewBuildHeader :
    Session
    -> Model
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
                            ++ Build.Styles.triggerButton
                                buttonDisabled
                                buttonHovered
                                build.status
                        )
                    <|
                        [ Icon.icon
                            { sizePx = 40
                            , image = Assets.AddCircleIcon |> Assets.CircleOutlineIcon
                            }
                            []
                        ]
                            ++ (if buttonDisabled && buttonHovered then
                                    [ Html.div
                                        (Build.Styles.buttonTooltip 240)
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
                        ++ Build.Styles.abortButton abortHovered
                    )
                    [ Icon.icon
                        { sizePx = 40
                        , image = Assets.AbortCircleIcon |> Assets.CircleOutlineIcon
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
                ++ Build.Styles.header build.status
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
            [ onWheel ScrollBuilds ]
            [ lazyViewHistory build model.history ]
        ]


tombstone : Time.Zone -> CurrentBuild -> List (Html Message)
tombstone timeZone currentBuild =
    let
        build =
            currentBuild.build

        maybeBirthDate =
            Maybe.Extra.or build.duration.startedAt build.duration.finishedAt
    in
    case ( maybeBirthDate, build.reapTime ) of
        ( Just birthDate, Just reapTime ) ->
            [ Html.div
                [ class "tombstone" ]
                [ Html.div [ class "heading" ] [ Html.text "RIP" ]
                , Html.div
                    [ class "job-name" ]
                    [ Html.text <|
                        Maybe.withDefault
                            "one-off build"
                        <|
                            Maybe.map .jobName build.job
                    ]
                , Html.div
                    [ class "build-name" ]
                    [ Html.text <|
                        "build #"
                            ++ (case build.job of
                                    Nothing ->
                                        String.fromInt build.id

                                    Just _ ->
                                        build.name
                               )
                    ]
                , Html.div
                    [ class "date" ]
                    [ Html.text <|
                        mmDDYY timeZone birthDate
                            ++ "-"
                            ++ mmDDYY timeZone reapTime
                    ]
                , Html.div
                    [ class "epitaph" ]
                    [ Html.text <|
                        case build.status of
                            Concourse.BuildStatus.BuildStatusSucceeded ->
                                "It passed, and now it has passed on."

                            Concourse.BuildStatus.BuildStatusFailed ->
                                "It failed, and now has been forgotten."

                            Concourse.BuildStatus.BuildStatusErrored ->
                                "It errored, but has found forgiveness."

                            Concourse.BuildStatus.BuildStatusAborted ->
                                "It was never given a chance."

                            _ ->
                                "I'm not dead yet."
                    ]
                ]
            , Html.div
                [ class "explanation" ]
                [ Html.text "This log has been "
                , Html.a
                    [ Html.Attributes.href "https://concourse-ci.org/jobs.html#job-build-log-retention" ]
                    [ Html.text "reaped." ]
                ]
            ]

        _ ->
            []


keyboardHelp : Bool -> Html Message
keyboardHelp showHelp =
    Html.div
        [ classList
            [ ( "keyboard-help", True )
            , ( "hidden", not showHelp )
            ]
        ]
        [ Html.div
            [ class "help-title" ]
            [ Html.text "keyboard shortcuts" ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span [ class "key" ] [ Html.text "h" ]
                , Html.span [ class "key" ] [ Html.text "l" ]
                ]
            , Html.text "previous/next build"
            ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span [ class "key" ] [ Html.text "j" ]
                , Html.span [ class "key" ] [ Html.text "k" ]
                ]
            , Html.text "scroll down/up"
            ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span [ class "key" ] [ Html.text "T" ] ]
            , Html.text "trigger a new build"
            ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span [ class "key" ] [ Html.text "A" ] ]
            , Html.text "abort build"
            ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span [ class "key" ] [ Html.text "gg" ] ]
            , Html.text "scroll to the top"
            ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span [ class "key" ] [ Html.text "G" ] ]
            , Html.text "scroll to the bottom"
            ]
        , Html.div
            [ class "help-line" ]
            [ Html.div
                [ class "keys" ]
                [ Html.span [ class "key" ] [ Html.text "?" ] ]
            , Html.text "hide/show help"
            ]
        ]


viewBuildOutput : Session -> CurrentOutput -> Html Message
viewBuildOutput session output =
    case output of
        Output o ->
            Build.Output.Output.view
                { timeZone = session.timeZone, hovered = session.hovered }
                o

        Cancelled ->
            Html.div
                Build.Styles.errorLog
                [ Html.text "build cancelled" ]

        Empty ->
            Html.div [] []


viewBuildPrep : Maybe Concourse.BuildPrep -> Html Message
viewBuildPrep buildPrep =
    case buildPrep of
        Just prep ->
            Html.div [ class "build-step" ]
                [ Html.div
                    [ class "header"
                    , style "display" "flex"
                    , style "align-items" "center"
                    ]
                    [ Icon.icon
                        { sizePx = 15, image = Assets.CogsIcon }
                        [ style "margin" "6.5px"
                        , style "margin-right" "0.5px"
                        , style "background-size" "contain"
                        ]
                    , Html.h3 [] [ Html.text "preparing build" ]
                    ]
                , Html.div []
                    [ Html.ul [ class "prep-status-list" ]
                        ([ viewBuildPrepLi "checking pipeline is not paused" prep.pausedPipeline Dict.empty
                         , viewBuildPrepLi "checking job is not paused" prep.pausedJob Dict.empty
                         ]
                            ++ viewBuildPrepInputs prep.inputs
                            ++ [ viewBuildPrepLi "waiting for a suitable set of input versions" prep.inputsSatisfied prep.missingInputReasons
                               , viewBuildPrepLi "checking max-in-flight is not reached" prep.maxRunningBuilds Dict.empty
                               ]
                        )
                    ]
                ]

        Nothing ->
            Html.div [] []


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
            ++ Build.Styles.historyItem
                currentBuild.status
                (build.id == currentBuild.id)
                build.status
        )
        [ Html.a
            [ onLeftClick <| Click <| BuildTab build.id build.name
            , href <| Routes.toString <| Routes.buildRoute build.id build.name build.job
            ]
            [ Html.text build.name ]
        ]


mmDDYY : Time.Zone -> Time.Posix -> String
mmDDYY =
    DateFormat.format
        [ DateFormat.monthFixed
        , DateFormat.text "/"
        , DateFormat.dayOfMonthFixed
        , DateFormat.text "/"
        , DateFormat.yearNumberLastTwo
        ]


viewBuildPrepLi :
    String
    -> Concourse.BuildPrepStatus
    -> Dict String String
    -> Html Message
viewBuildPrepLi text status details =
    Html.li
        [ classList
            [ ( "prep-status", True )
            , ( "inactive", status == Concourse.BuildPrepStatusUnknown )
            ]
        ]
        [ Html.div
            [ style "align-items" "center"
            , style "display" "flex"
            ]
            [ viewBuildPrepStatus status
            , Html.span []
                [ Html.text text ]
            ]
        , viewBuildPrepDetails details
        ]


viewBuildPrepInputs : Dict String Concourse.BuildPrepStatus -> List (Html Message)
viewBuildPrepInputs inputs =
    List.map viewBuildPrepInput (Dict.toList inputs)


viewBuildPrepInput : ( String, Concourse.BuildPrepStatus ) -> Html Message
viewBuildPrepInput ( name, status ) =
    viewBuildPrepLi ("discovering any new versions of " ++ name) status Dict.empty


viewBuildPrepDetails : Dict String String -> Html Message
viewBuildPrepDetails details =
    Html.ul [ class "details" ]
        (List.map viewDetailItem (Dict.toList details))


viewBuildPrepStatus : Concourse.BuildPrepStatus -> Html Message
viewBuildPrepStatus status =
    case status of
        Concourse.BuildPrepStatusUnknown ->
            Html.div
                [ title "thinking..." ]
                [ Spinner.spinner
                    { sizePx = 12
                    , margin = "0 5px 0 0"
                    }
                ]

        Concourse.BuildPrepStatusBlocking ->
            Html.div
                [ title "blocking" ]
                [ Spinner.spinner
                    { sizePx = 12
                    , margin = "0 5px 0 0"
                    }
                ]

        Concourse.BuildPrepStatusNotBlocking ->
            Icon.icon
                { sizePx = 12
                , image = Assets.NotBlockingCheckIcon
                }
                [ style "margin-right" "5px"
                , style "background-size" "contain"
                , title "not blocking"
                ]


viewDetailItem : ( String, String ) -> Html Message
viewDetailItem ( name, status ) =
    Html.li []
        [ Html.text (name ++ " - " ++ status) ]


sampleSession : Session
sampleSession =
    { authToken = ""
    , clusterName = ""
    , csrfToken = ""
    , expandedTeamsInAllPipelines = Set.empty
    , collapsedTeamsInFavorites = Set.empty
    , favoritedPipelines = Set.empty
    , hovered = HoverState.NoHover
    , sideBarState =
        { isOpen = False
        , width = 275
        }
    , draggingSideBar = False
    , notFoundImgSrc = ""
    , pipelineRunningKeyframes = ""
    , pipelines = RemoteData.NotAsked
    , screenSize = ScreenSize.Desktop
    , timeZone = Time.utc
    , turbulenceImgSrc = ""
    , userState = UserState.UserStateLoggedOut
    , version = ""
    }


sampleOldModel : Model
sampleOldModel =
    { page = OneOffBuildPage 0
    , now = Nothing
    , disableManualTrigger = False
    , history = []
    , nextPage = Nothing
    , currentBuild =
        RemoteData.Success
            { build =
                { id = 0
                , name = "0"
                , job = Nothing
                , status = Concourse.BuildStatus.BuildStatusStarted
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Nothing
                    }
                , reapTime = Nothing
                }
            , prep = Nothing
            , output =
                Output
                    { steps = steps
                    , state = Build.Output.Models.StepsLiveUpdating
                    , eventSourceOpened = True
                    , eventStreamUrlPath = Nothing
                    , highlight = Routes.HighlightNothing
                    }
            }
    , autoScroll = True
    , previousKeyPress = Nothing
    , shiftDown = False
    , isTriggerBuildKeyDown = False
    , showHelp = False
    , highlight = Routes.HighlightNothing
    , hoveredCounter = 0
    , fetchingHistory = False
    , scrolledToCurrentBuild = True
    , authorized = True
    , isUserMenuExpanded = False
    }


sampleModel : Build.Models.Model
sampleModel =
    { page = OneOffBuildPage 0
    , id = 0
    , name = "0"
    , now = Nothing
    , job = Nothing
    , disableManualTrigger = False
    , history = []
    , nextPage = Nothing
    , prep = Nothing
    , duration = { startedAt = Nothing, finishedAt = Nothing }
    , status = Concourse.BuildStatus.BuildStatusStarted
    , output =
        Output
            { steps = steps
            , state = Build.Output.Models.StepsLiveUpdating
            , eventSourceOpened = True
            , eventStreamUrlPath = Nothing
            , highlight = Routes.HighlightNothing
            }
    , autoScroll = True
    , isScrollToIdInProgress = False
    , previousKeyPress = Nothing
    , isTriggerBuildKeyDown = False
    , showHelp = False
    , highlight = Routes.HighlightNothing
    , authorized = True
    , fetchingHistory = False
    , scrolledToCurrentBuild = False
    , shiftDown = False
    , isUserMenuExpanded = False
    , hasLoadedYet = True
    , notFound = False
    , reapTime = Nothing
    }


ansiLogStyle : Ansi.Log.Style
ansiLogStyle =
    { foreground = Nothing
    , background = Nothing
    , bold = False
    , faint = False
    , italic = False
    , underline = False
    , blink = False
    , inverted = False
    , fraktur = False
    , framed = False
    }


position : Ansi.Log.CursorPosition
position =
    { row = 0
    , column = 0
    }


log : Ansi.Log.Model
log =
    { lineDiscipline = Ansi.Log.Cooked
    , lines = Array.empty
    , position = position
    , savedPosition = Nothing
    , style = ansiLogStyle
    , remainder = ""
    }


tree : STModels.StepTree
tree =
    STModels.Task
        { id = "stepid"
        , name = "task_step"
        , state = STModels.StepStateRunning
        , log = log
        , error = Nothing
        , expanded = True
        , version = Nothing
        , metadata = []
        , changed = False
        , timestamps = Dict.empty
        , initialize = Nothing
        , start = Nothing
        , finish = Nothing
        }


steps : Maybe STModels.StepTreeModel
steps =
    Just
        { tree = tree
        , foci = Dict.empty
        , highlight = Routes.HighlightNothing
        }


sampleJob : String -> List String -> Concourse.Job
sampleJob name passed =
    { name = name
    , pipelineName = "pipeline"
    , teamName = "team"
    , nextBuild = Nothing
    , finishedBuild = Nothing
    , transitionBuild = Nothing
    , paused = False
    , disableManualTrigger = False
    , inputs =
        [ { name = "input"
          , resource = "resource"
          , passed = passed
          , trigger = True
          }
        ]
    , outputs = []
    , groups = []
    }


sampleJobs : List Concourse.Job
sampleJobs =
    [ sampleJob "job1" []
    , sampleJob "job2a" [ "job1" ]
    , sampleJob "job2b" [ "job1" ]
    , sampleJob "job3" [ "job2a" ]
    , sampleJob "job4" [ "job3" ]
    ]


dashboardPreviewView : List Concourse.Job -> Html msg
dashboardPreviewView jobs =
    let
        groups =
            jobGroups jobs

        width =
            Dict.size groups

        height =
            Maybe.withDefault 0 <| List.maximum (List.map List.length (Dict.values groups))
    in
    Html.div
        [ classList
            [ ( "pipeline-grid", True )
            , ( "pipeline-grid-wide", width > 12 )
            , ( "pipeline-grid-tall", height > 12 )
            , ( "pipeline-grid-super-wide", width > 24 )
            , ( "pipeline-grid-super-tall", height > 24 )
            ]
        ]
    <|
        List.map
            (\js ->
                List.map viewJob js
                    |> Html.div [ class "parallel-grid" ]
            )
            (Dict.values groups)


viewJob : Concourse.Job -> Html msg
viewJob job =
    let
        jobStatus =
            case job.finishedBuild of
                Just fb ->
                    Concourse.BuildStatus.show fb.status

                Nothing ->
                    "no-builds"

        isJobRunning =
            job.nextBuild /= Nothing

        latestBuild =
            if job.nextBuild == Nothing then
                job.finishedBuild

            else
                job.nextBuild
    in
    Html.div
        [ classList
            [ ( "node " ++ jobStatus, True )
            , ( "running", isJobRunning )
            , ( "paused", job.paused )
            ]
        , attribute "data-tooltip" job.name
        ]
    <|
        case latestBuild of
            Nothing ->
                [ Html.a [ href <| Routes.toString <| Routes.jobRoute job ] [ Html.text "" ] ]

            Just build ->
                [ Html.a [ href <| Routes.toString <| Routes.buildRoute build.id build.name build.job ] [ Html.text "" ] ]


jobGroups : List Concourse.Job -> Dict Int (List Concourse.Job)
jobGroups jobs =
    let
        jobLookup =
            jobByName <| List.foldl (\job byName -> Dict.insert job.name job byName) Dict.empty jobs
    in
    Dict.foldl
        (\jobName depth byDepth ->
            Dict.update depth
                (\jobsA ->
                    Just (jobLookup jobName :: Maybe.withDefault [] jobsA)
                )
                byDepth
        )
        Dict.empty
        (jobDepths jobs Dict.empty)


jobByName : Dict String Concourse.Job -> String -> Concourse.Job
jobByName jobs job =
    case Dict.get job jobs of
        Just a ->
            a

        Nothing ->
            { name = ""
            , pipelineName = ""
            , teamName = ""
            , nextBuild = Nothing
            , finishedBuild = Nothing
            , transitionBuild = Nothing
            , paused = False
            , disableManualTrigger = False
            , inputs = []
            , outputs = []
            , groups = []
            }


jobDepths : List Concourse.Job -> Dict String Int -> Dict String Int
jobDepths jobs dict =
    case jobs of
        [] ->
            dict

        job :: otherJobs ->
            let
                passedJobs =
                    List.concatMap .passed job.inputs
            in
            case List.length passedJobs of
                0 ->
                    jobDepths otherJobs <| Dict.insert job.name 0 dict

                _ ->
                    let
                        passedJobDepths =
                            List.map (\passedJob -> Dict.get passedJob dict) passedJobs
                    in
                    if List.member Nothing passedJobDepths then
                        jobDepths (List.append otherJobs [ job ]) dict

                    else
                        let
                            depths =
                                List.map (\depth -> Maybe.withDefault 0 depth) passedJobDepths

                            maxPassedJobDepth =
                                Maybe.withDefault 0 <| List.maximum depths
                        in
                        jobDepths otherJobs <| Dict.insert job.name (maxPassedJobDepth + 1) dict
