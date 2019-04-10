module Dashboard.DashboardPreview exposing (view)

import Concourse
import Concourse.PipelineStatus exposing (PipelineStatus(..), StatusDetails(..))
import Dashboard.Styles as Styles
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import List.Extra exposing (find)
import Message.Message exposing (DomID(..), Message(..))
import Routes
import TopologicalSort exposing (flattenToLayers)


view : Maybe DomID -> List Concourse.Job -> Html Message
view hovered jobs =
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
        (List.map (viewJobLayer hovered) layers)


viewJobLayer : Maybe DomID -> List Concourse.Job -> Html Message
viewJobLayer hovered jobs =
    Html.div [ class "parallel-grid" ] (List.map (viewJob hovered) jobs)


viewJob : Maybe DomID -> Concourse.Job -> Html Message
viewJob hovered job =
    let
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

        jobId =
            { jobName = job.name
            , pipelineName = job.pipelineName
            , teamName = job.teamName
            }
    in
    Html.div
        (attribute "data-tooltip" job.name
            :: Styles.jobPreview job (hovered == (Just <| JobPreview jobId))
            ++ [ onMouseEnter <| Hover <| Just <| JobPreview jobId
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Html.a
            (href (Routes.toString buildRoute) :: Styles.jobPreviewLink)
            [ Html.text "" ]
        ]
