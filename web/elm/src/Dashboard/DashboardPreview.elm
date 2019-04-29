module Dashboard.DashboardPreview exposing (view)

import Concourse
import Concourse.BuildStatus
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href)
import List.Extra exposing (find)
import Routes
import TopologicalSort exposing (flattenToLayers)


view : List Concourse.Job -> Html msg
view jobs =
    let
        jobDependencies : Concourse.Job -> List Concourse.Job
        jobDependencies job =
            job.inputs
                |> List.concatMap .passed
                |> List.filterMap (\name -> find (\j -> j.name == name) jobs)

        layers : List (List Concourse.Job)
        layers =
            flattenToLayers (List.map (\j -> ( j, jobDependencies j )) jobs)

        width : Int
        width =
            List.length layers

        height : Int
        height =
            layers
                |> List.map List.length
                |> List.maximum
                |> Maybe.withDefault 0
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
        (List.map viewJobLayer layers)


viewJobLayer : List Concourse.Job -> Html msg
viewJobLayer jobs =
    Html.div [ class "parallel-grid" ] (List.map viewJob jobs)


viewJob : Concourse.Job -> Html msg
viewJob job =
    let
        jobStatus : String
        jobStatus =
            job.finishedBuild
                |> Maybe.map .status
                |> Maybe.map Concourse.BuildStatus.show
                |> Maybe.withDefault "no-builds"

        latestBuild : Maybe Concourse.Build
        latestBuild =
            if job.nextBuild == Nothing then
                job.finishedBuild

            else
                job.nextBuild

        buildRoute : Routes.Route
        buildRoute =
            case latestBuild of
                Nothing ->
                    Routes.jobRoute job

                Just build ->
                    Routes.buildRoute build
    in
    Html.div
        [ classList
            [ ( "node " ++ jobStatus, True )
            , ( "running", job.nextBuild /= Nothing )
            , ( "paused", job.paused )
            ]
        , attribute "data-tooltip" job.name
        ]
        [ Html.a [ href <| Routes.toString buildRoute ] [ Html.text "" ] ]
