module Dashboard.DashboardPreview exposing (groupByRank, view)

import Concourse
import Concourse.PipelineStatus exposing (PipelineStatus(..), StatusDetails(..))
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import HoverState
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, href)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import List.Extra
import Message.Message exposing (DomID(..), Message(..), PipelinesSection(..))
import Routes


view : PipelinesSection -> HoverState.HoverState -> List (List Concourse.Job) -> Html Message
view section hovered layers =
    Html.div
        (class "pipeline-grid" :: Styles.pipelinePreviewGrid)
        (List.map (viewJobLayer section hovered) layers)


viewJobLayer : PipelinesSection -> HoverState.HoverState -> List Concourse.Job -> Html Message
viewJobLayer section hovered jobs =
    Html.div [ class "parallel-grid" ] (List.map (viewJob section hovered) jobs)


viewJob : PipelinesSection -> HoverState.HoverState -> Concourse.Job -> Html Message
viewJob section hovered job =
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
                    Routes.buildRoute build.id build.name build.job

        jobId =
            { jobName = job.name
            , pipelineId = job.pipelineId
            }
    in
    Html.div
        (attribute "data-tooltip" job.name
            :: Styles.jobPreview job
                (HoverState.isHovered
                    (JobPreview section jobId)
                    hovered
                )
            ++ [ onMouseEnter <| Hover <| Just <| JobPreview section jobId
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
