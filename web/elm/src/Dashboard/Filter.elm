module Dashboard.Filter exposing (Suggestion, filterTeams, isViewingInstanceGroups, suggestions)

import Application.Models exposing (Session)
import Concourse exposing (flattenJson)
import Concourse.PipelineStatus
    exposing
        ( PipelineStatus(..)
        , StatusDetails(..)
        , equal
        , isRunning
        )
import Dashboard.Group.Models exposing (Card(..), Pipeline)
import Dashboard.Models exposing (Model)
import Dashboard.Pipeline as Pipeline
import Dict exposing (Dict)
import Favorites
import FetchResult
import List.Extra
import Parser
    exposing
        ( (|.)
        , (|=)
        , Parser
        , Step(..)
        , backtrackable
        , chompUntilEndOr
        , chompWhile
        , end
        , getChompedString
        , getOffset
        , getSource
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
import Simple.Fuzzy


type alias Filter =
    { negate : Bool
    , teamFilter : TeamFilter
    }


filterTypes : List String
filterTypes =
    [ "status", "team", "group" ]


type TeamFilter
    = Team StringFilter
    | Pipeline PipelineFilter
    | InstanceGroup StringFilter


type PipelineFilter
    = Name StringFilter
    | Status StatusFilter


type StringFilter
    = Fuzzy String
    | Exact String
    | StartsWith String


type StatusFilter
    = PipelineStatus PipelineStatus
    | PipelineRunning
    | IncompleteStatus String


filterTeams : Session -> Model -> Dict String (List Pipeline)
filterTeams session { pipelineJobs, jobs, query, teams, pipelines, dashboardView } =
    let
        teamsToFilter =
            teams
                |> FetchResult.withDefault []
                |> List.map (\t -> ( t.name, [] ))
                |> Dict.fromList
                |> Dict.union (pipelines |> Maybe.withDefault Dict.empty)
                |> Dict.map
                    (\_ p ->
                        List.filter (prefilter session dashboardView) p
                    )
    in
    parseFilters query
        |> List.map Tuple.first
        |> List.foldr (runFilter (FetchResult.withDefault Dict.empty jobs) pipelineJobs) teamsToFilter


prefilter : Session -> Routes.DashboardView -> Pipeline -> Bool
prefilter session view p =
    case view of
        Routes.ViewNonArchivedPipelines ->
            not p.archived || Favorites.isPipelineFavorited session p

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
    case f.teamFilter of
        Team sf ->
            Dict.filter (\team _ -> stringMatches sf team |> negater)

        Pipeline pf ->
            Dict.map
                (\_ pipelines -> List.filter (pipelineFilter pf jobs existingJobs >> negater) pipelines)
                >> Dict.filter (\_ pipelines -> not <| List.isEmpty pipelines)

        InstanceGroup sf ->
            Dict.map
                (\_ ->
                    Concourse.groupPipelinesWithinTeam
                        >> List.filterMap
                            (\g ->
                                case g of
                                    Concourse.InstanceGroup p ps ->
                                        Just (p :: ps)

                                    _ ->
                                        Nothing
                            )
                        >> List.concatMap identity
                        >> List.filter (.name >> stringMatches sf >> negater)
                )
                >> Dict.filter (\_ groups -> not <| List.isEmpty groups)


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
        Name sf ->
            let
                instanceVarValues =
                    pipeline.instanceVars
                        |> Dict.toList
                        |> List.concatMap (\( k, v ) -> flattenJson k v)
                        |> List.map Tuple.second
            in
            List.any (stringMatches sf) (pipeline.name :: instanceVarValues)

        Status sf ->
            case sf of
                PipelineStatus ps ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> equal ps

                PipelineRunning ->
                    pipeline |> Pipeline.pipelineStatus jobsForPipeline |> isRunning

                IncompleteStatus _ ->
                    False


stringMatches : StringFilter -> String -> Bool
stringMatches f =
    case f of
        Fuzzy term ->
            Simple.Fuzzy.match term

        Exact name ->
            (==) name

        StartsWith prefix ->
            String.startsWith prefix


parseFilters : String -> List ( Filter, String )
parseFilters =
    run
        (loop [] <|
            \revFilters ->
                oneOf
                    [ end |> map (\_ -> Done (List.reverse revFilters))
                    , filter |> captureChompedString |> map (\f -> Loop (f :: revFilters))
                    ]
        )
        >> Result.withDefault []


captureChompedString : Parser a -> Parser ( a, String )
captureChompedString parser =
    succeed (\start val end source -> ( val, String.slice start end source ))
        |= getOffset
        |= parser
        |= getOffset
        |= getSource


filter : Parser Filter
filter =
    succeed Filter
        |. spaces
        |= oneOf
            [ symbol "-" |> map (always True)
            , succeed False
            ]
        |= teamFilter


teamFilter : Parser TeamFilter
teamFilter =
    oneOf
        [ backtrackable (keyedStringFilter "team" |> map Team)
        , backtrackable (keyedStringFilter "group" |> map InstanceGroup)
        , backtrackable statusFilter
        , succeed (Name >> Pipeline) |= parseString
        ]


parseString : Parser StringFilter
parseString =
    oneOf
        [ parseQuotedString
        , parseWord |> map Fuzzy
        ]


parseQuotedString : Parser StringFilter
parseQuotedString =
    getChompedString
        (symbol "\""
            |. chompUntilEndOr "\""
            |. oneOf [ symbol "\"", end ]
        )
        |> map
            (\s ->
                if String.endsWith "\"" s && s /= "\"" then
                    Exact <| String.slice 1 -1 s

                else
                    StartsWith <| String.dropLeft 1 s
            )


parseWord : Parser String
parseWord =
    getChompedString
        (chompWhile
            (\c -> c /= ' ' && c /= '\t' && c /= '\n' && c /= '\u{000D}')
        )


keyedStringFilter : String -> Parser StringFilter
keyedStringFilter key =
    succeed identity
        |. keyword key
        |. symbol ":"
        |. spaces
        |= parseString


statusFilter : Parser TeamFilter
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


type alias Suggestion =
    { prev : String
    , cur : String
    }


suggestions : Dict String (List Pipeline) -> String -> List Suggestion
suggestions pipelines query =
    let
        parsedFilters =
            parseFilters query

        ( curFilter, negated ) =
            parsedFilters
                |> List.Extra.last
                |> Maybe.map Tuple.first
                |> Maybe.map (\f -> ( f.teamFilter, f.negate ))
                |> Maybe.withDefault ( Pipeline (Name (Fuzzy "")), False )

        prevFilters =
            parsedFilters
                |> List.map Tuple.second
                |> List.reverse
                |> List.drop 1
                |> List.reverse

        prev =
            if List.isEmpty prevFilters then
                ""

            else
                String.join "" prevFilters ++ " "

        cur =
            case curFilter of
                Pipeline (Name (Fuzzy s)) ->
                    filterTypes
                        -- As long as instanced pipelines are experimental,
                        -- lets not suggest the "group:" filter. Note that it
                        -- can still be applied, and group suggestions will
                        -- appear when you explicitly type "group:"
                        |> List.filter ((/=) "group")
                        |> List.filter (String.startsWith s)
                        |> List.map (\v -> v ++ ":")

                Pipeline (Name _) ->
                    []

                Pipeline (Status sf) ->
                    case sf of
                        IncompleteStatus status ->
                            [ "paused", "pending", "failed", "errored", "aborted", "running", "succeeded" ]
                                |> List.filter (String.startsWith status)
                                |> List.map (\v -> "status:" ++ v)

                        _ ->
                            []

                InstanceGroup (Exact _) ->
                    []

                InstanceGroup _ ->
                    pipelines
                        |> Dict.values
                        |> List.concat
                        |> List.map .name
                        |> List.Extra.unique
                        |> List.map (\v -> "group:" ++ quoted v)

                Team (Exact _) ->
                    []

                Team _ ->
                    pipelines
                        |> Dict.keys
                        |> List.take 10
                        |> List.map (\v -> "team:" ++ quoted v)

        prefix =
            if negated then
                List.map (\c -> "-" ++ c)

            else
                identity
    in
    List.map (Suggestion prev) (prefix cur)


quoted : String -> String
quoted s =
    "\"" ++ s ++ "\""


isViewingInstanceGroups : String -> Bool
isViewingInstanceGroups query =
    parseFilters query
        |> List.map Tuple.first
        |> List.any
            (\f ->
                case f.teamFilter of
                    InstanceGroup _ ->
                        True

                    _ ->
                        False
            )
