module Dashboard.Filter exposing (filterTeams, suggestions)

import Concourse exposing (DatabaseID, hyphenNotation)
import Concourse.PipelineStatus
    exposing
        ( PipelineStatus(..)
        , StatusDetails(..)
        , equal
        , isRunning
        )
import Dashboard.Group.Models exposing (Card(..), Pipeline)
import Dashboard.Pipeline as Pipeline
import Dict exposing (Dict)
import FetchResult exposing (FetchResult)
import List.Extra
import Parser
    exposing
        ( (|.)
        , (|=)
        , Parser
        , Step(..)
        , backtrackable
        , chompWhile
        , end
        , getChompedString
        , keyword
        , loop
        , map
        , oneOf
        , run
        , spaces
        , succeed
        , symbol
        )
import Routes
import Set exposing (Set)
import Simple.Fuzzy


type alias Filter =
    { negate : Bool
    , groupFilter : GroupFilter
    }


type GroupFilter
    = Team String
    | Pipeline PipelineFilter


type PipelineFilter
    = FuzzyName String
    | Status StatusFilter


type StatusFilter
    = PipelineStatus PipelineStatus
    | PipelineRunning
    | IncompleteStatus String


filterTypes : List String
filterTypes =
    [ "status", "team" ]


filterTeams :
    { pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobName)
    , jobs : Dict ( Concourse.DatabaseID, Concourse.JobName ) Concourse.Job
    , query : String
    , teams : List Concourse.Team
    , pipelines : Dict String (List Pipeline)
    , dashboardView : Routes.DashboardView
    , favoritedPipelines : Set DatabaseID
    }
    -> Dict String (List Pipeline)
filterTeams { pipelineJobs, jobs, query, teams, pipelines, dashboardView, favoritedPipelines } =
    let
        teamsToFilter =
            teams
                |> List.map (\t -> ( t.name, [] ))
                |> Dict.fromList
                |> Dict.union pipelines
                |> Dict.map
                    (\_ p ->
                        List.filter (prefilter dashboardView favoritedPipelines) p
                    )
    in
    parseFilters query |> List.foldr (runFilter jobs pipelineJobs) teamsToFilter


prefilter : Routes.DashboardView -> Set DatabaseID -> Pipeline -> Bool
prefilter view favoritedPipelines p =
    case view of
        Routes.ViewNonArchivedPipelines ->
            not p.archived || Set.member p.id favoritedPipelines

        _ ->
            True


runFilter :
    Dict ( Concourse.DatabaseID, Concourse.JobName ) Concourse.Job
    -> Dict Concourse.DatabaseID (List Concourse.JobName)
    -> Filter
    -> Dict String (List Pipeline)
    -> Dict String (List Pipeline)
runFilter jobs existingJobs f =
    let
        negater =
            if f.negate then
                not

            else
                identity
    in
    case f.groupFilter of
        Team teamName ->
            Dict.filter (\team _ -> team |> Simple.Fuzzy.match teamName |> negater)

        Pipeline pf ->
            Dict.map
                (\_ pipelines -> List.filter (pipelineFilter pf jobs existingJobs >> negater) pipelines)
                >> Dict.filter (\_ pipelines -> not <| List.isEmpty pipelines)


pipelineFilter :
    PipelineFilter
    -> Dict ( Concourse.DatabaseID, Concourse.JobName ) Concourse.Job
    -> Dict Concourse.DatabaseID (List Concourse.JobName)
    -> Pipeline
    -> Bool
pipelineFilter pf jobs existingJobs pipeline =
    let
        jobsForPipeline =
            existingJobs
                |> Dict.get pipeline.id
                |> Maybe.withDefault []
                |> List.filterMap (\j -> Dict.get ( pipeline.id, j ) jobs)
    in
    case pf of
        FuzzyName term ->
            Simple.Fuzzy.match term (pipeline.name ++ hyphenNotation pipeline.instanceVars)

        Status sf ->
            case sf of
                PipelineStatus ps ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> equal ps

                PipelineRunning ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> isRunning

                IncompleteStatus _ ->
                    False


parseFilters : String -> List Filter
parseFilters =
    run
        (loop [] <|
            \revFilters ->
                oneOf
                    [ end
                        |> map (\_ -> Done (List.reverse revFilters))
                    , filter
                        |> map (\f -> Loop (f :: revFilters))
                    ]
        )
        >> Result.withDefault []


filter : Parser Filter
filter =
    succeed Filter
        |. spaces
        |= oneOf
            [ symbol "-" |> map (always True)
            , succeed False
            ]
        |= groupFilter


groupFilter : Parser GroupFilter
groupFilter =
    oneOf
        [ backtrackable teamFilter
        , backtrackable statusFilter
        , succeed (FuzzyName >> Pipeline) |= parseWord
        ]


parseWord : Parser String
parseWord =
    getChompedString
        (chompWhile
            (\c -> c /= ' ' && c /= '\t' && c /= '\n' && c /= '\u{000D}')
        )


teamFilter : Parser GroupFilter
teamFilter =
    succeed Team
        |. keyword "team"
        |. symbol ":"
        |. spaces
        |= parseWord


statusFilter : Parser GroupFilter
statusFilter =
    succeed (Status >> Pipeline)
        |. keyword "status"
        |. symbol ":"
        |. spaces
        |= pipelineStatus


pipelineStatus : Parser StatusFilter
pipelineStatus =
    oneOf
        [ keyword "paused" |> map (\_ -> PipelineStatus PipelineStatusPaused)
        , keyword "aborted" |> map (\_ -> PipelineStatus <| PipelineStatusAborted Running)
        , keyword "errored" |> map (\_ -> PipelineStatus <| PipelineStatusErrored Running)
        , keyword "failed" |> map (\_ -> PipelineStatus <| PipelineStatusFailed Running)
        , keyword "pending" |> map (\_ -> PipelineStatus <| PipelineStatusPending False)
        , keyword "succeeded" |> map (\_ -> PipelineStatus <| PipelineStatusSucceeded Running)
        , keyword "running" |> map (\_ -> PipelineRunning)
        , parseWord |> map IncompleteStatus
        ]


suggestions :
    { a
        | query : String
        , teams : FetchResult (List Concourse.Team)
        , pipelines : Maybe (Dict String (List Pipeline))
    }
    -> List String
suggestions { query, teams, pipelines } =
    let
        lastFilter =
            parseFilters query
                |> List.Extra.last
                |> Maybe.map .groupFilter
                |> Maybe.withDefault (Pipeline (FuzzyName ""))
    in
    case lastFilter of
        Pipeline (FuzzyName s) ->
            filterTypes
                |> List.filter (String.startsWith s)
                |> List.map (\v -> v ++ ": ")

        Pipeline (Status sf) ->
            case sf of
                IncompleteStatus status ->
                    [ "paused", "pending", "failed", "errored", "aborted", "running", "succeeded" ]
                        |> List.filter (String.startsWith status)
                        |> List.map (\v -> "status: " ++ v)

                _ ->
                    []

        Team team ->
            Set.union
                (teams
                    |> FetchResult.withDefault []
                    |> List.map .name
                    |> Set.fromList
                )
                (pipelines
                    |> Maybe.withDefault Dict.empty
                    |> Dict.keys
                    |> Set.fromList
                )
                |> Set.toList
                |> List.filter (String.startsWith team)
                |> List.filter ((/=) team)
                |> List.take 10
                |> List.map (\v -> "team: " ++ v)
