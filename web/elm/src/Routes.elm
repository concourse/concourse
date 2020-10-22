module Routes exposing
    ( DashboardView(..)
    , Highlight(..)
    , Route(..)
    , SearchType(..)
    , StepID
    , Transition
    , buildRoute
    , extractInstanceGroup
    , extractPid
    , extractQuery
    , instanceGroupQueryParams
    , jobRoute
    , parsePath
    , pipelineRoute
    , searchQueryParams
    , showHighlight
    , toString
    , tokenToFlyRoute
    )

import Api.Pagination
import Concourse exposing (InstanceVars)
import Concourse.Pagination as Pagination exposing (Direction(..))
import Dict
import DotNotation
import Maybe.Extra
import RouteBuilder exposing (RouteBuilder, appendPath, appendQuery)
import Url
import Url.Builder as Builder
import Url.Parser
    exposing
        ( (</>)
        , (<?>)
        , Parser
        , custom
        , fragment
        , int
        , map
        , oneOf
        , parse
        , s
        , string
        , top
        )
import Url.Parser.Query as Query


type Route
    = Build { id : Concourse.JobBuildIdentifier, highlight : Highlight }
    | Resource { id : Concourse.ResourceIdentifier, page : Maybe Pagination.Page }
    | Job { id : Concourse.JobIdentifier, page : Maybe Pagination.Page }
    | OneOffBuild { id : Concourse.BuildId, highlight : Highlight }
    | Pipeline { id : Concourse.PipelineIdentifier, groups : List String }
    | Dashboard { searchType : SearchType, dashboardView : DashboardView }
    | FlySuccess Bool (Maybe Int)


type SearchType
    = HighDensity
    | Normal String (Maybe Concourse.InstanceGroupIdentifier)


type DashboardView
    = ViewNonArchivedPipelines
    | ViewAllPipelines


dashboardViews : List DashboardView
dashboardViews =
    [ ViewNonArchivedPipelines, ViewAllPipelines ]


dashboardViewName : DashboardView -> String
dashboardViewName view =
    case view of
        ViewAllPipelines ->
            "all"

        ViewNonArchivedPipelines ->
            "non_archived"


type Highlight
    = HighlightNothing
    | HighlightLine StepID Int
    | HighlightRange StepID Int Int


type alias StepID =
    String


type alias Transition =
    { from : Route
    , to : Route
    }



-- pages


pipelineIdentifier : Parser ({ teamName : String, pipelineName : String } -> a) a
pipelineIdentifier =
    s "teams"
        </> string
        </> s "pipelines"
        </> string
        |> map
            (\t p ->
                { teamName = t
                , pipelineName = p
                }
            )


build : Parser ((InstanceVars -> Route) -> a) a
build =
    let
        buildHelper { teamName, pipelineName } jobName buildName h =
            \iv ->
                Build
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        , jobName = jobName
                        , buildName = buildName
                        }
                    , highlight = h
                    }
    in
    map buildHelper
        (pipelineIdentifier
            </> s "jobs"
            </> string
            </> s "builds"
            </> string
            </> fragment parseHighlight
        )


oneOffBuild : Parser ((b -> Route) -> a) a
oneOffBuild =
    map
        (\b h -> always <| OneOffBuild { id = b, highlight = h })
        (s "builds" </> int </> fragment parseHighlight)


parsePage : Maybe Int -> Maybe Int -> Maybe Int -> Maybe Pagination.Page
parsePage from to limit =
    case ( from, to, limit ) of
        ( Nothing, Just t, Just l ) ->
            Just
                { direction = Pagination.To t
                , limit = l
                }

        ( Just f, Nothing, Just l ) ->
            Just
                { direction = Pagination.From f
                , limit = l
                }

        _ ->
            Nothing


resource : Parser ((InstanceVars -> Route) -> a) a
resource =
    let
        resourceHelper { teamName, pipelineName } resourceName from to limit =
            \iv ->
                Resource
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        , resourceName = resourceName
                        }
                    , page = parsePage from to limit
                    }
    in
    map resourceHelper
        (pipelineIdentifier
            </> s "resources"
            </> string
            <?> Query.int "from"
            <?> Query.int "to"
            <?> Query.int "limit"
        )


job : Parser ((InstanceVars -> Route) -> a) a
job =
    let
        jobHelper { teamName, pipelineName } jobName from to limit =
            \iv ->
                Job
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        , jobName = jobName
                        }
                    , page = parsePage from to limit
                    }
    in
    map jobHelper
        (pipelineIdentifier
            </> s "jobs"
            </> string
            <?> Query.int "from"
            <?> Query.int "to"
            <?> Query.int "limit"
        )


pipeline : Parser ((InstanceVars -> Route) -> a) a
pipeline =
    map
        (\{ teamName, pipelineName } g ->
            \iv ->
                Pipeline
                    { id =
                        { teamName = teamName
                        , pipelineName = pipelineName
                        , pipelineInstanceVars = iv
                        }
                    , groups = g
                    }
        )
        (pipelineIdentifier <?> Query.custom "group" identity)


dashboard : Parser ((b -> Route) -> a) a
dashboard =
    map (\st view -> always <| Dashboard { searchType = st, dashboardView = view }) <|
        oneOf
            [ (top
                <?> (stringWithSpaces "search" |> Query.map (Maybe.withDefault ""))
                <?> instanceGroupQuery
              )
                |> map Normal
            , s "hd" |> map HighDensity
            ]
            <?> dashboardViewQuery


dashboardViewQuery : Query.Parser DashboardView
dashboardViewQuery =
    (Query.enum "view" <|
        Dict.fromList
            (dashboardViews
                |> List.map (\v -> ( dashboardViewName v, v ))
            )
    )
        |> Query.map (Maybe.withDefault ViewNonArchivedPipelines)


instanceGroupQuery : Query.Parser (Maybe Concourse.InstanceGroupIdentifier)
instanceGroupQuery =
    Query.map2
        (\t g ->
            case ( t, g ) of
                ( Just teamName, Just groupName ) ->
                    Just { teamName = teamName, name = groupName }

                _ ->
                    Nothing
        )
        (stringWithSpaces "team")
        (stringWithSpaces "group")


stringWithSpaces : String -> Query.Parser (Maybe String)
stringWithSpaces =
    -- https://github.com/elm/url/issues/32
    Query.string >> Query.map (Maybe.map (String.replace "+" " "))


flySuccess : Parser ((b -> Route) -> a) a
flySuccess =
    map (\s p -> always <| FlySuccess (s == Just "true") p)
        (s "fly_success"
            <?> Query.string "noop"
            <?> Query.int "fly_port"
        )



-- route utils


buildRoute : Int -> String -> Maybe Concourse.JobIdentifier -> Route
buildRoute id name jobId =
    case jobId of
        Just j ->
            Build
                { id =
                    { teamName = j.teamName
                    , pipelineName = j.pipelineName
                    , pipelineInstanceVars = j.pipelineInstanceVars
                    , jobName = j.jobName
                    , buildName = name
                    }
                , highlight = HighlightNothing
                }

        Nothing ->
            OneOffBuild { id = id, highlight = HighlightNothing }


jobRoute : Concourse.Job -> Route
jobRoute j =
    Job
        { id =
            { teamName = j.teamName
            , pipelineName = j.pipelineName
            , pipelineInstanceVars = j.pipelineInstanceVars
            , jobName = j.name
            }
        , page = Nothing
        }


pipelineRoute : { a | name : String, teamName : String, instanceVars : InstanceVars } -> Route
pipelineRoute p =
    Pipeline
        { id = Concourse.toPipelineId p
        , groups = []
        }


showHighlight : Highlight -> String
showHighlight hl =
    case hl of
        HighlightNothing ->
            ""

        HighlightLine id line ->
            "#L" ++ id ++ ":" ++ String.fromInt line

        HighlightRange id line1 line2 ->
            "#L"
                ++ id
                ++ ":"
                ++ String.fromInt line1
                ++ ":"
                ++ String.fromInt line2


parseHighlight : Maybe String -> Highlight
parseHighlight hash =
    case hash of
        Just h ->
            case String.uncons h of
                Just ( 'L', selector ) ->
                    case String.split ":" selector of
                        [ stepID, line1str, line2str ] ->
                            case ( String.toInt line1str, String.toInt line2str ) of
                                ( Just line1, Just line2 ) ->
                                    HighlightRange stepID line1 line2

                                _ ->
                                    HighlightNothing

                        [ stepID, linestr ] ->
                            case String.toInt linestr of
                                Just line ->
                                    HighlightLine stepID line

                                _ ->
                                    HighlightNothing

                        _ ->
                            HighlightNothing

                _ ->
                    HighlightNothing

        _ ->
            HighlightNothing


tokenToFlyRoute : String -> Int -> String
tokenToFlyRoute authToken flyPort =
    Builder.crossOrigin
        ("http://127.0.0.1:" ++ String.fromInt flyPort)
        []
        [ Builder.string "token" authToken ]



-- router


sitemap : Parser ((InstanceVars -> Route) -> a) a
sitemap =
    oneOf
        [ resource
        , job
        , dashboard
        , pipeline
        , build
        , oneOffBuild
        , flySuccess
        ]


toString : Route -> String
toString route =
    case route of
        Build { id, highlight } ->
            (pipelineIdBuilder id
                |> appendPath [ "jobs", id.jobName, "builds", id.buildName ]
                |> RouteBuilder.build
            )
                ++ showHighlight highlight

        Job { id, page } ->
            pipelineIdBuilder id
                |> appendPath [ "jobs", id.jobName ]
                |> appendQuery (Api.Pagination.params page)
                |> RouteBuilder.build

        Resource { id, page } ->
            pipelineIdBuilder id
                |> appendPath [ "resources", id.resourceName ]
                |> appendQuery (Api.Pagination.params page)
                |> RouteBuilder.build

        OneOffBuild { id, highlight } ->
            (( [ "builds", String.fromInt id ], [] )
                |> RouteBuilder.build
            )
                ++ showHighlight highlight

        Pipeline { id, groups } ->
            pipelineIdBuilder id
                |> appendQuery (groups |> List.map (Builder.string "group"))
                |> RouteBuilder.build

        Dashboard { searchType, dashboardView } ->
            ( [], [] )
                |> appendPath
                    (case searchType of
                        Normal _ _ ->
                            []

                        HighDensity ->
                            [ "hd" ]
                    )
                |> appendQuery
                    (case searchType of
                        Normal "" Nothing ->
                            []

                        Normal "" (Just ig) ->
                            instanceGroupQueryParams ig

                        Normal query Nothing ->
                            searchQueryParams query

                        Normal query (Just ig) ->
                            searchQueryParams query ++ instanceGroupQueryParams ig

                        _ ->
                            []
                    )
                |> appendQuery
                    (case dashboardView of
                        ViewNonArchivedPipelines ->
                            []

                        _ ->
                            [ Builder.string "view" <| dashboardViewName dashboardView ]
                    )
                |> RouteBuilder.build

        FlySuccess noop flyPort ->
            ( [ "fly_success" ], [] )
                |> appendQuery
                    (flyPort
                        |> Maybe.map (Builder.int "fly_port")
                        |> Maybe.Extra.toList
                    )
                |> appendQuery
                    (if noop then
                        [ Builder.string "noop" "true" ]

                     else
                        []
                    )
                |> RouteBuilder.build


parsePath : Url.Url -> Maybe Route
parsePath url =
    let
        instanceVars =
            url.query
                |> Maybe.withDefault ""
                |> String.split "&"
                |> List.filterMap (removePrefix "var.")
                |> List.filterMap Url.percentDecode
                |> List.filterMap (DotNotation.parse >> Result.toMaybe)
                |> DotNotation.expand
    in
    parse sitemap url |> Maybe.map (\deferredRoute -> deferredRoute instanceVars)


removePrefix : String -> String -> Maybe String
removePrefix prefix s =
    if String.startsWith prefix s then
        Just <| String.dropLeft (String.length prefix) s

    else
        Nothing



-- route utils


extractPid : Route -> Maybe Concourse.PipelineIdentifier
extractPid route =
    case route of
        Build { id } ->
            Just <| Concourse.pipelineId id

        Job { id } ->
            Just <| Concourse.pipelineId id

        Resource { id } ->
            Just <| Concourse.pipelineId id

        Pipeline { id } ->
            Just id

        _ ->
            Nothing


extractQuery : SearchType -> String
extractQuery route =
    case route of
        Normal q _ ->
            q

        _ ->
            ""


extractInstanceGroup : SearchType -> Maybe Concourse.InstanceGroupIdentifier
extractInstanceGroup route =
    case route of
        Normal _ ig ->
            ig

        _ ->
            Nothing


instanceGroupQueryParams : Concourse.InstanceGroupIdentifier -> List Builder.QueryParameter
instanceGroupQueryParams { teamName, name } =
    [ Builder.string "team" teamName, Builder.string "group" name ]


searchQueryParams : String -> List Builder.QueryParameter
searchQueryParams q =
    [ Builder.string "search" q ]


pipelineIdBuilder : { r | teamName : String, pipelineName : String, pipelineInstanceVars : Concourse.InstanceVars } -> RouteBuilder
pipelineIdBuilder id =
    ( [ "teams", id.teamName, "pipelines", id.pipelineName ]
    , DotNotation.flatten id.pipelineInstanceVars
        |> List.map
            (\var ->
                let
                    ( k, v ) =
                        DotNotation.serialize var
                in
                Builder.string ("var." ++ k) v
            )
    )
