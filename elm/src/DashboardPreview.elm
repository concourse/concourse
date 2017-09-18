module DashboardPreview exposing (view)

import Concourse
import Concourse.BuildStatus
import Debug
import Dict exposing (Dict)
import Html exposing (Html)
import Html.Attributes exposing (class, title)


view : List Concourse.Job -> List (Html msg)
view jobs =
    let
        groups =
            Dict.values <| jobGroups jobs
    in
        List.map (\jobs -> Html.div [ class "parallel-grid" ] <| List.map viewJob jobs) groups


viewJob : Concourse.Job -> Html msg
viewJob job =
    let
        linkAttrs =
            case job.finishedBuild of
                Just fb ->
                    Concourse.BuildStatus.show fb.status

                Nothing ->
                    "no-builds"
    in
        Html.div [ class ("node job " ++ linkAttrs), title job.name ] [ Html.text "" ]


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
