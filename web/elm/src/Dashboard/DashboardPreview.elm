module Dashboard.DashboardPreview exposing (groupByRank, view)

import Concourse
import Concourse.PipelineStatus exposing (PipelineStatus(..), StatusDetails(..))
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import List.Extra
import Message.Message exposing (DomID(..), Message(..))
import Routes


view : HoverState.HoverState -> List Concourse.Job -> Html Message
view hovered jobs =
    let
        layers : List (List Concourse.Job)
        layers =
            groupByRank jobs

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


viewJobLayer : HoverState.HoverState -> List Concourse.Job -> Html Message
viewJobLayer hovered jobs =
    Html.div [ class "parallel-grid" ] (List.map (viewJob hovered) jobs)


viewJob : HoverState.HoverState -> Concourse.Job -> Html Message
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
            :: Styles.jobPreview job (HoverState.isHovered (JobPreview jobId) hovered)
            ++ [ onMouseEnter <| Hover <| Just <| JobPreview jobId
               , onMouseLeave <| Hover Nothing
               ]
        )
        [ Html.a
            (href (Routes.toString buildRoute) :: Styles.jobPreviewLink)
            [ Html.text "" ]
        ]


type alias Job a b =
    { a
        | name : String
        , inputs : List { b | passed : List String }
    }


groupByRank : List (Job a b) -> List (List (Job a b))
groupByRank jobs =
    let
        depths =
            jobDepths Dict.empty Dict.empty jobs
    in
    depths
        |> Dict.values
        |> List.sort
        |> List.Extra.unique
        |> List.map
            (\d ->
                jobs
                    |> List.filter (\j -> Dict.get j.name depths == Just d)
            )


jobDepths :
    Dict String { value : Int, uncertainty : Int }
    -> Dict String Int
    -> List (Job a b)
    -> Dict String Int
jobDepths calculations depths jobs =
    case jobs of
        [] ->
            depths

        job :: otherJobs ->
            let
                dependencies =
                    List.concatMap .passed job.inputs

                values =
                    List.filterMap
                        (\jobName -> Dict.get jobName depths)
                        dependencies

                new =
                    { value =
                        values
                            |> List.maximum
                            |> Maybe.map ((+) 1)
                            |> Maybe.withDefault 0
                    , uncertainty = List.length otherJobs
                    }

                totalConfidence =
                    List.length values
                        == List.length dependencies

                neverGonnaGetBetter =
                    Dict.get job.name calculations
                        |> Maybe.map (\oldCalc -> oldCalc.uncertainty <= new.uncertainty)
                        |> Maybe.withDefault False
            in
            if totalConfidence || neverGonnaGetBetter then
                jobDepths
                    (Dict.remove job.name calculations)
                    (Dict.insert job.name new.value depths)
                    otherJobs

            else
                jobDepths
                    (Dict.insert job.name new calculations)
                    depths
                    (otherJobs ++ [ job ])
