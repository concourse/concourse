module BuildTests exposing (all)

import Build
import Build.Effects as Effects
import Build.Msgs as Msgs
import Concourse
import DashboardTests exposing (defineHoverBehaviour, iconSelector, middleGrey)
import Expect
import Html.Attributes as Attr
import Test exposing (..)
import Test.Html.Query as Query
import Test.Html.Selector exposing (attribute, id, style, text)


all : Test
all =
    describe "build page" <|
        let
            pageLoad =
                Build.init
                    { title = always Cmd.none
                    }
                    { csrfToken = ""
                    , hash = ""
                    }
                    (Build.JobBuildPage
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        , buildName = "1"
                        }
                    )

            theBuild : Concourse.Build
            theBuild =
                { id = 1
                , name = "1"
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        }
                , status = Concourse.BuildStatusSucceeded
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Nothing
                    }
                , reapTime = Nothing
                }

            pendingBuild : Concourse.Build
            pendingBuild =
                { id = 1
                , name = "1"
                , job =
                    Just
                        { teamName = "team"
                        , pipelineName = "pipeline"
                        , jobName = "job"
                        }
                , status = Concourse.BuildStatusPending
                , duration =
                    { startedAt = Nothing
                    , finishedAt = Nothing
                    }
                , reapTime = Nothing
                }

            fetchBuild : Build.Model -> ( Build.Model, List Effects.Effect )
            fetchBuild =
                Build.update <| Msgs.BuildFetched 1 <| Ok theBuild

            fetchPendingBuild : Build.Model -> ( Build.Model, List Effects.Effect )
            fetchPendingBuild =
                Build.update <| Msgs.BuildFetched 1 <| Ok pendingBuild

            fetchJobDetails : Build.Model -> ( Build.Model, List Effects.Effect )
            fetchJobDetails =
                Build.update <|
                    Msgs.BuildJobDetailsFetched <|
                        Ok
                            { pipeline =
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                }
                            , name = "job"
                            , pipelineName = "pipeline"
                            , teamName = "team"
                            , nextBuild = Nothing
                            , finishedBuild = Nothing
                            , transitionBuild = Nothing
                            , paused = False
                            , disableManualTrigger = False
                            , inputs = []
                            , outputs = []
                            , groups = []
                            }

            fetchHistory : Build.Model -> ( Build.Model, List Effects.Effect )
            fetchHistory =
                Build.update
                    (Msgs.BuildHistoryFetched
                        (Ok
                            { pagination =
                                { previousPage = Nothing
                                , nextPage = Nothing
                                }
                            , content = [ theBuild ]
                            }
                        )
                    )
        in
        [ test "says loading on page load" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.has [ text "loading" ]
        , test "fetches build on page load" <|
            \_ ->
                pageLoad
                    |> Tuple.second
                    |> Expect.equal
                        [ Effects.FetchJobBuild 1
                            { teamName = "team"
                            , pipelineName = "pipeline"
                            , jobName = "job"
                            , buildName = "1"
                            }
                        , Effects.GetCurrentTime
                        ]
        , test "has a header after the build is fetched" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.has [ id "build-header" ]
        , test "fetches build history and job details after build is fetched" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.second
                    |> Expect.all
                        [ List.member
                            (Effects.FetchBuildHistory
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , jobName = "job"
                                }
                                Nothing
                            )
                            >> Expect.true
                                "expected effect was not in the list"
                        , List.member
                            (Effects.FetchBuildJobDetails
                                { teamName = "team"
                                , pipelineName = "pipeline"
                                , jobName = "job"
                                }
                            )
                            >> Expect.true
                                "expected effect was not in the list"
                        ]
        , test "header lays out horizontally" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find [ id "build-header" ]
                    |> Query.has
                        [ style [ ( "display", "flex" ) ] ]
        , test "header spreads out contents" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find [ id "build-header" ]
                    |> Query.has
                        [ style [ ( "justify-content", "space-between" ) ] ]
        , test
            ("trigger build button on right side of header "
                ++ "after history and job details fetched"
            )
          <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find [ id "build-header" ]
                    |> Query.children []
                    |> Query.index -1
                    |> Query.has
                        [ attribute <|
                            Attr.attribute "aria-label" "Trigger Build"
                        ]
        , test "trigger build button has dark grey background" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Trigger Build"
                        ]
                    |> Query.has
                        [ style
                            [ ( "padding", "10px" )
                            , ( "border", "none" )
                            , ( "background-color", middleGrey )
                            , ( "outline", "none" )
                            ]
                        ]
        , test "trigger build button has pointer cursor" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Trigger Build"
                        ]
                    |> Query.has [ style [ ( "cursor", "pointer" ) ] ]
        , test "trigger build button has 'plus' icon" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Trigger Build"
                        ]
                    |> Query.children []
                    |> Query.first
                    |> Query.has
                        (iconSelector
                            { size = "40px"
                            , image = "ic_add_circle_outline_white.svg"
                            }
                        )
        , defineHoverBehaviour
            { name = "trigger build button"
            , setup =
                pageLoad
                    |> Tuple.first
                    |> fetchBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
            , query =
                Build.view
                    >> Query.fromHtml
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Trigger Build"
                        ]
            , updateFunc = \msg -> Build.update msg >> Tuple.first
            , unhoveredSelector =
                { description = "grey plus icon"
                , selector =
                    [ style [ ( "opacity", "0.5" ) ] ]
                        ++ iconSelector
                            { size = "40px"
                            , image = "ic_add_circle_outline_white.svg"
                            }
                }
            , hoveredSelector =
                { description = "white plus icon"
                , selector =
                    [ style [ ( "opacity", "1" ) ] ]
                        ++ iconSelector
                            { size = "40px"
                            , image = "ic_add_circle_outline_white.svg"
                            }
                }
            , mouseEnterMsg = Msgs.Hover Msgs.Trigger
            , mouseLeaveMsg = Msgs.Hover Msgs.Neither
            }
        , test "build action section lays out horizontally" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchPendingBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find [ id "build-header" ]
                    |> Query.children []
                    |> Query.index -1
                    |> Query.has [ style [ ( "display", "flex" ) ] ]
        , test "abort build button is to the left of the trigger button" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchPendingBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find [ id "build-header" ]
                    |> Query.children []
                    |> Query.index -1
                    |> Query.children []
                    |> Query.first
                    |> Query.has
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
        , test "abort build button has dark grey background" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchPendingBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    |> Query.has
                        [ style
                            [ ( "padding", "10px" )
                            , ( "border", "none" )
                            , ( "background-color", middleGrey )
                            , ( "outline", "none" )
                            ]
                        ]
        , test "abort build button has pointer cursor" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchPendingBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    |> Query.has [ style [ ( "cursor", "pointer" ) ] ]
        , test "abort build button has 'X' icon" <|
            \_ ->
                pageLoad
                    |> Tuple.first
                    |> fetchPendingBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
                    |> Build.view
                    |> Query.fromHtml
                    |> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
                    |> Query.children []
                    |> Query.first
                    |> Query.has
                        (iconSelector
                            { size = "40px"
                            , image = "ic_abort_circle_outline_white.svg"
                            }
                        )
        , defineHoverBehaviour
            { name = "abort build button"
            , setup =
                pageLoad
                    |> Tuple.first
                    |> fetchPendingBuild
                    |> Tuple.first
                    |> fetchHistory
                    |> Tuple.first
                    |> fetchJobDetails
                    |> Tuple.first
            , query =
                Build.view
                    >> Query.fromHtml
                    >> Query.find
                        [ attribute <|
                            Attr.attribute "aria-label" "Abort Build"
                        ]
            , updateFunc = \msg -> Build.update msg >> Tuple.first
            , unhoveredSelector =
                { description = "grey abort icon"
                , selector =
                    [ style [ ( "opacity", "0.5" ) ] ]
                        ++ iconSelector
                            { size = "40px"
                            , image = "ic_abort_circle_outline_white.svg"
                            }
                }
            , hoveredSelector =
                { description = "white abort icon"
                , selector =
                    [ style [ ( "opacity", "1" ) ] ]
                        ++ iconSelector
                            { size = "40px"
                            , image = "ic_abort_circle_outline_white.svg"
                            }
                }
            , mouseEnterMsg = Msgs.Hover Msgs.Abort
            , mouseLeaveMsg = Msgs.Hover Msgs.Neither
            }
        ]
