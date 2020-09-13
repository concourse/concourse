module Dashboard.Filter exposing (filterTeams)

import Concourse exposing (DatabaseID)
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


filterTeams :
    { pipelineJobs : Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
    , jobs : Dict ( Concourse.DatabaseID, String ) Concourse.Job
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
    Dict ( Concourse.DatabaseID, String ) Concourse.Job
    -> Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
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


lookupJob : Dict ( Concourse.DatabaseID, String ) Concourse.Job -> Concourse.JobIdentifier -> Maybe Concourse.Job
lookupJob jobs jobId =
    jobs
        |> Dict.get ( jobId.pipelineId, jobId.jobName )


pipelineFilter :
    PipelineFilter
    -> Dict ( Concourse.DatabaseID, String ) Concourse.Job
    -> Dict Concourse.DatabaseID (List Concourse.JobIdentifier)
    -> Pipeline
    -> Bool
pipelineFilter pf jobs existingJobs pipeline =
    let
        jobsForPipeline =
            existingJobs
                |> Dict.get pipeline.id
                |> Maybe.withDefault []
                |> List.filterMap (lookupJob jobs)
    in
    case pf of
        FuzzyName term ->
            Simple.Fuzzy.match term pipeline.name

        Status sf ->
            case sf of
                PipelineStatus ps ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> equal ps

                PipelineRunning ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> isRunning


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
    oneOf
        [ succeed (Filter True) |. spaces |. symbol "-" |= groupFilter |. spaces
        , succeed (Filter False) |. spaces |= groupFilter |. spaces
        ]


type GroupFilter
    = Team String
    | Pipeline PipelineFilter


type PipelineFilter
    = FuzzyName String
    | Status StatusFilter


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


type StatusFilter
    = PipelineStatus PipelineStatus
    | PipelineRunning


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
        [ map (\_ -> PipelineStatus PipelineStatusPaused) (keyword "paused")
        , map (\_ -> PipelineStatus <| PipelineStatusAborted Running)
            (keyword "aborted")
        , map (\_ -> PipelineStatus <| PipelineStatusErrored Running)
            (keyword "errored")
        , map (\_ -> PipelineStatus <| PipelineStatusFailed Running)
            (keyword "failed")
        , map (\_ -> PipelineStatus <| PipelineStatusPending False)
            (keyword "pending")
        , map (\_ -> PipelineStatus <| PipelineStatusSucceeded Running)
            (keyword "succeeded")
        , map (\_ -> PipelineRunning) (keyword "running")
        ]
