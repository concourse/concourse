module Dashboard.DashboardPreview exposing (view)

import Concourse
import Concourse.PipelineStatus exposing (PipelineStatus(..), StatusDetails(..))
import Dashboard.Styles as Styles
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href)
import Html.Events exposing (onMouseEnter, onMouseLeave)
import List.Extra
import Maybe.Extra
import Message.Message exposing (DomID(..), Message(..))
import Routes
import Set exposing (Set)


view : Maybe DomID -> List Concourse.Job -> Html Message
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


groupByRank : List Concourse.Job -> List (List Concourse.Job)
groupByRank jobs =
    let
        depths =
            jobs
                |> jobDepths Set.empty Dict.empty
    in
    jobs
        |> List.Extra.gatherEqualsBy (\job -> Dict.get job.name depths)
        |> List.map (\( h, t ) -> h :: t)


jobDepths : Set String -> Dict String Int -> List Concourse.Job -> Dict String Int
jobDepths visited depths jobs =
    case jobs of
        [] ->
            depths

        job :: otherJobs ->
            case
                ( List.concatMap .passed job.inputs
                    |> List.map (\jobName -> Dict.get jobName depths)
                    |> calculate List.maximum
                , Set.member job.name visited
                )
            of
                ( Nothing, _ ) ->
                    jobDepths visited (Dict.insert job.name 0 depths) otherJobs

                ( Just (Confident depth), _ ) ->
                    jobDepths visited (Dict.insert job.name (depth + 1) depths) otherJobs

                ( Just (Speculative depth), True ) ->
                    jobDepths visited (Dict.insert job.name (depth + 1) depths) otherJobs

                ( Just (Speculative _), False ) ->
                    jobDepths (Set.insert job.name visited) depths (otherJobs ++ [ job ])


type Calculation a
    = Confident a
    | Speculative a


calculate : (List a -> Maybe b) -> List (Maybe a) -> Maybe (Calculation b)
calculate f xs =
    case ( List.any ((==) Nothing) xs, f (Maybe.Extra.values xs) ) of
        ( True, Just value ) ->
            Just (Speculative value)

        ( False, Just value ) ->
            Just (Confident value)

        ( _, Nothing ) ->
            Nothing
