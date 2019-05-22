module Dashboard.DashboardPreview exposing (view)

import Concourse
import Concourse.BuildStatus
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href)
import List.Extra
import Maybe.Extra
import Routes
import Set exposing (Set)


view : List Concourse.Job -> Html msg
view jobs =
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
