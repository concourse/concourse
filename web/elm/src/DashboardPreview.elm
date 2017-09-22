module DashboardPreview exposing (view)

import Concourse
import Concourse.BuildStatus
import Debug
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (attribute, class, classList, href)


view : List Concourse.Job -> Html msg
view jobs =
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
                (\jobs ->
                    List.map viewJob jobs
                        |> Html.div [ class "parallel-grid" ]
                )
                (Dict.values groups)


viewJob : Concourse.Job -> Html msg
viewJob job =
    let
        linkAttrs =
            case job.finishedBuild of
                Just fb ->
                    Concourse.BuildStatus.show fb.status

                Nothing ->
                    "no-builds"

        latestBuild =
            if job.nextBuild == Nothing then
                job.finishedBuild
            else
                job.nextBuild
    in
        Html.div [ class ("node " ++ linkAttrs), attribute "data-tooltip" job.name ] <|
            case latestBuild of
                Nothing ->
                    [ Html.text "" ]

                Just build ->
                    [ Html.a [ href build.url ] [ Html.text "" ] ]


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
            Debug.crash "impossible"


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
