module Pipeline exposing
    ( Flags
    , Model
    , changeToPipelineAndGroups
    , getUpdateMessage
    , handleCallback
    , init
    , subscriptions
    , update
    , view
    )

import Char
import Colors
import Concourse
import Concourse.Cli
import Effects exposing (Callback(..), Effect(..))
import Html exposing (Html)
import Html.Attributes exposing (class, height, href, id, src, style, width)
import Html.Attributes.Aria exposing (ariaLabel)
import Http
import Json.Decode
import Json.Encode
import Keyboard
import Mouse
import Pipeline.Msgs exposing (Msg(..))
import QueryString
import RemoteData exposing (..)
import Routes
import StrictEvents exposing (onLeftClickOrShiftLeftClick)
import Svg exposing (..)
import Svg.Attributes as SvgAttributes
import Time exposing (Time)
import UpdateMsg exposing (UpdateMsg)


type alias Model =
    { pipelineLocator : Concourse.PipelineIdentifier
    , pipeline : WebData Concourse.Pipeline
    , fetchedJobs : Maybe Json.Encode.Value
    , fetchedResources : Maybe Json.Encode.Value
    , renderedJobs : Maybe Json.Encode.Value
    , renderedResources : Maybe Json.Encode.Value
    , concourseVersion : String
    , turbulenceImgSrc : String
    , experiencingTurbulence : Bool
    , route : Routes.ConcourseRoute
    , selectedGroups : List String
    , hideLegend : Bool
    , hideLegendCounter : Time
    }


type alias Flags =
    { teamName : String
    , pipelineName : String
    , turbulenceImgSrc : String
    , route : Routes.ConcourseRoute
    }


queryGroupsForRoute : Routes.ConcourseRoute -> List String
queryGroupsForRoute route =
    QueryString.all "groups" route.queries


init : Flags -> ( Model, List Effect )
init flags =
    let
        pipelineLocator =
            { teamName = flags.teamName
            , pipelineName = flags.pipelineName
            }

        model =
            { concourseVersion = ""
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , pipelineLocator = pipelineLocator
            , pipeline = RemoteData.NotAsked
            , fetchedJobs = Nothing
            , fetchedResources = Nothing
            , renderedJobs = Nothing
            , renderedResources = Nothing
            , experiencingTurbulence = False
            , route = flags.route
            , selectedGroups = queryGroupsForRoute flags.route
            , hideLegend = False
            , hideLegendCounter = 0
            }
    in
    loadPipeline pipelineLocator model


changeToPipelineAndGroups : Flags -> Model -> ( Model, Cmd Callback )
changeToPipelineAndGroups flags model =
    let
        pid =
            { teamName = flags.teamName
            , pipelineName = flags.pipelineName
            }

        ( newModel, effects ) =
            if model.pipelineLocator == pid then
                renderIfNeeded
                    { model | selectedGroups = queryGroupsForRoute flags.route }

            else
                init flags
    in
    ( newModel, Cmd.batch <| List.map Effects.runEffect effects )


loadPipeline : Concourse.PipelineIdentifier -> Model -> ( Model, List Effect )
loadPipeline pipelineLocator model =
    ( { model | pipelineLocator = pipelineLocator }
    , [ FetchPipeline pipelineLocator, FetchVersion, ResetPipelineFocus ]
    )


timeUntilHidden : Time
timeUntilHidden =
    10 * Time.second


timeUntilHiddenCheckInterval : Time
timeUntilHiddenCheckInterval =
    1 * Time.second


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    case model.pipeline of
        RemoteData.Failure _ ->
            UpdateMsg.NotFound

        _ ->
            UpdateMsg.AOK


handleCallback : Callback -> Model -> ( Model, List Effect )
handleCallback callback model =
    let
        redirectToLoginIfUnauthenticated status =
            if status.code == 401 then
                [ RedirectToLogin ]

            else
                []
    in
    case callback of
        PipelineFetched (Ok pipeline) ->
            ( { model | pipeline = RemoteData.Success pipeline }
            , [ FetchJobs model.pipelineLocator
              , FetchResources model.pipelineLocator
              , SetTitle <| pipeline.name ++ " - "
              ]
            )

        PipelineFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | pipeline = RemoteData.Failure err }, [] )

                    else
                        ( model, redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded { model | experiencingTurbulence = True }

        JobsFetched (Ok fetchedJobs) ->
            renderIfNeeded { model | fetchedJobs = Just fetchedJobs, experiencingTurbulence = False }

        JobsFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model, redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded { model | fetchedJobs = Nothing, experiencingTurbulence = True }

        ResourcesFetched (Ok fetchedResources) ->
            renderIfNeeded { model | fetchedResources = Just fetchedResources, experiencingTurbulence = False }

        ResourcesFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model, redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded { model | fetchedResources = Nothing, experiencingTurbulence = True }

        VersionFetched (Ok version) ->
            ( { model | concourseVersion = version, experiencingTurbulence = False }, [] )

        VersionFetched (Err err) ->
            flip always (Debug.log "failed to fetch version" err) <|
                ( { model | experiencingTurbulence = True }, [] )

        _ ->
            ( model, [] )


update : Msg -> Model -> ( Model, List Effect )
update msg model =
    let
        redirectToLoginIfUnauthenticated status =
            if status.code == 401 then
                [ RedirectToLogin ]

            else
                []
    in
    case msg of
        Noop ->
            ( model, [] )

        HideLegendTimerTicked _ ->
            if model.hideLegendCounter + timeUntilHiddenCheckInterval > timeUntilHidden then
                ( { model | hideLegend = True }, [] )

            else
                ( { model | hideLegendCounter = model.hideLegendCounter + timeUntilHiddenCheckInterval }
                , []
                )

        ShowLegend ->
            ( { model | hideLegend = False, hideLegendCounter = 0 }, [] )

        KeyPressed keycode ->
            if (Char.fromCode keycode |> Char.toLower) == 'f' then
                ( model, [ ResetPipelineFocus ] )

            else
                ( model, [] )

        AutoupdateTimerTicked timestamp ->
            ( model, [ FetchPipeline model.pipelineLocator ] )

        PipelineIdentifierFetched pipelineIdentifier ->
            ( model, [ FetchPipeline pipelineIdentifier ] )

        AutoupdateVersionTicked _ ->
            ( model, [ FetchVersion ] )

        ToggleGroup group ->
            ( model, [ NewUrl <| getNextUrl (toggleGroup group model.selectedGroups model.pipeline) model ] )

        SetGroups groups ->
            ( model, [ NewUrl <| getNextUrl groups model ] )


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.batch
        [ autoupdateVersionTimer
        , Time.every (5 * Time.second) AutoupdateTimerTicked
        , Time.every timeUntilHiddenCheckInterval HideLegendTimerTicked
        , Mouse.moves (\_ -> ShowLegend)
        , Keyboard.presses (\_ -> ShowLegend)
        , Mouse.clicks (\_ -> ShowLegend)
        , Keyboard.presses KeyPressed
        ]


view : Model -> Html Msg
view model =
    Html.div [ class "pipeline-view" ]
        [ Html.nav
            [ class "groups-bar" ]
            [ viewGroupsBar model ]
        , Html.div [ class "pipeline-content" ]
            [ Svg.svg
                [ SvgAttributes.class "pipeline-graph test" ]
                []
            , Html.div
                [ if model.experiencingTurbulence then
                    class "error-message"

                  else
                    class "error-message hidden"
                ]
                [ Html.div [ class "message" ]
                    [ Html.img [ src model.turbulenceImgSrc, class "seatbelt" ] []
                    , Html.p [] [ Html.text "experiencing turbulence" ]
                    , Html.p [ class "explanation" ] []
                    ]
                ]
            , Html.dl
                [ if model.hideLegend then
                    class "legend hidden"

                  else
                    class "legend"
                ]
                [ Html.dt [ class "succeeded" ] []
                , Html.dd [] [ Html.text "succeeded" ]
                , Html.dt [ class "errored" ] []
                , Html.dd [] [ Html.text "errored" ]
                , Html.dt [ class "aborted" ] []
                , Html.dd [] [ Html.text "aborted" ]
                , Html.dt [ class "paused" ] []
                , Html.dd [] [ Html.text "paused" ]
                , Html.dt [ Html.Attributes.style [ ( "background-color", Colors.pinned ) ] ] []
                , Html.dd [] [ Html.text "pinned" ]
                , Html.dt [ class "failed" ] []
                , Html.dd [] [ Html.text "failed" ]
                , Html.dt [ class "pending" ] []
                , Html.dd [] [ Html.text "pending" ]
                , Html.dt [ class "started" ] []
                , Html.dd [] [ Html.text "started" ]
                , Html.dt [ class "dotted" ] [ Html.text "." ]
                , Html.dd [] [ Html.text "dependency" ]
                , Html.dt [ class "solid" ] [ Html.text "-" ]
                , Html.dd [] [ Html.text "dependency (trigger)" ]
                ]
            , Html.table [ class "lower-right-info" ]
                [ Html.tr []
                    [ Html.td [ class "label" ] [ Html.text "cli:" ]
                    , Html.td []
                        [ Html.ul [ class "cli-downloads" ]
                            [ Html.li []
                                [ Html.a
                                    [ href <|
                                        Concourse.Cli.downloadUrl
                                            "amd64"
                                            "darwin"
                                    , ariaLabel "Download OS X CLI"
                                    , Html.Attributes.style <|
                                        cliIcon "apple"
                                    ]
                                    []
                                ]
                            , Html.li []
                                [ Html.a
                                    [ href <|
                                        Concourse.Cli.downloadUrl
                                            "amd64"
                                            "windows"
                                    , ariaLabel "Download Windows CLI"
                                    , Html.Attributes.style <|
                                        cliIcon "windows"
                                    ]
                                    []
                                ]
                            , Html.li []
                                [ Html.a
                                    [ href <|
                                        Concourse.Cli.downloadUrl
                                            "amd64"
                                            "linux"
                                    , ariaLabel "Download Linux CLI"
                                    , Html.Attributes.style <|
                                        cliIcon "linux"
                                    ]
                                    []
                                ]
                            ]
                        ]
                    ]
                , Html.tr []
                    [ Html.td [ class "label" ] [ Html.text "version:" ]
                    , Html.td []
                        [ Html.div [ id "concourse-version" ]
                            [ Html.text "v"
                            , Html.span [ class "number" ] [ Html.text model.concourseVersion ]
                            ]
                        ]
                    ]
                ]
            ]
        ]


viewGroupsBar : Model -> Html Msg
viewGroupsBar model =
    let
        groupList =
            case model.pipeline of
                RemoteData.Success pipeline ->
                    List.map
                        (viewGroup (getSelectedGroupsForRoute model) (Routes.pipelineRoute pipeline))
                        pipeline.groups

                _ ->
                    []

        groupClass =
            if List.isEmpty groupList then
                "hidden"

            else
                "groups"
    in
    Html.ul [ class groupClass ] groupList


viewGroup : List String -> String -> Concourse.PipelineGroup -> Html Msg
viewGroup selectedGroups url grp =
    Html.li
        [ if List.member grp.name selectedGroups then
            class "main active"

          else
            class "main"
        ]
        [ Html.a
            [ Html.Attributes.href <| url ++ "?groups=" ++ grp.name
            , onLeftClickOrShiftLeftClick (SetGroups [ grp.name ]) (ToggleGroup grp)
            ]
            [ Html.text grp.name ]
        ]


autoupdateVersionTimer : Sub Msg
autoupdateVersionTimer =
    Time.every (1 * Time.minute) AutoupdateVersionTicked


jobAppearsInGroups : List String -> Concourse.PipelineIdentifier -> Json.Encode.Value -> Bool
jobAppearsInGroups groupNames pi jobJson =
    let
        concourseJob =
            Json.Decode.decodeValue (Concourse.decodeJob pi) jobJson
    in
    case concourseJob of
        Ok cj ->
            anyIntersect cj.groups groupNames

        Err err ->
            flip always (Debug.log "failed to check if job is in group" err) <|
                False


expandJsonList : Json.Encode.Value -> List Json.Decode.Value
expandJsonList flatList =
    let
        result =
            Json.Decode.decodeValue (Json.Decode.list Json.Decode.value) flatList
    in
    case result of
        Ok res ->
            res

        Err err ->
            []


filterJobs : Model -> Json.Encode.Value -> Json.Encode.Value
filterJobs model value =
    Json.Encode.list <|
        List.filter
            (jobAppearsInGroups (activeGroups model) model.pipelineLocator)
            (expandJsonList value)


activeGroups : Model -> List String
activeGroups model =
    case ( model.selectedGroups, model.pipeline |> RemoteData.toMaybe |> Maybe.andThen (List.head << .groups) ) of
        ( [], Just firstGroup ) ->
            [ firstGroup.name ]

        ( groups, _ ) ->
            groups


renderIfNeeded : Model -> ( Model, List Effect )
renderIfNeeded model =
    case ( model.fetchedResources, model.fetchedJobs ) of
        ( Just fetchedResources, Just fetchedJobs ) ->
            let
                filteredFetchedJobs =
                    if List.isEmpty (activeGroups model) then
                        fetchedJobs

                    else
                        filterJobs model fetchedJobs
            in
            case ( model.renderedResources, model.renderedJobs ) of
                ( Just renderedResources, Just renderedJobs ) ->
                    if
                        (expandJsonList renderedJobs /= expandJsonList filteredFetchedJobs)
                            || (expandJsonList renderedResources /= expandJsonList fetchedResources)
                    then
                        ( { model
                            | renderedJobs = Just filteredFetchedJobs
                            , renderedResources = Just fetchedResources
                          }
                        , [ RenderPipeline filteredFetchedJobs fetchedResources ]
                        )

                    else
                        ( model, [] )

                _ ->
                    ( { model
                        | renderedJobs = Just filteredFetchedJobs
                        , renderedResources = Just fetchedResources
                      }
                    , [ RenderPipeline filteredFetchedJobs fetchedResources ]
                    )

        _ ->
            ( model, [] )


anyIntersect : List a -> List a -> Bool
anyIntersect list1 list2 =
    case list1 of
        [] ->
            False

        first :: rest ->
            if List.member first list2 then
                True

            else
                anyIntersect rest list2


toggleGroup : Concourse.PipelineGroup -> List String -> WebData Concourse.Pipeline -> List String
toggleGroup grp names mpipeline =
    if List.member grp.name names then
        List.filter ((/=) grp.name) names

    else if List.isEmpty names then
        grp.name :: getDefaultSelectedGroups mpipeline

    else
        grp.name :: names


getSelectedGroupsForRoute : Model -> List String
getSelectedGroupsForRoute model =
    if List.isEmpty model.selectedGroups then
        getDefaultSelectedGroups model.pipeline

    else
        model.selectedGroups


getDefaultSelectedGroups : WebData Concourse.Pipeline -> List String
getDefaultSelectedGroups pipeline =
    case pipeline of
        RemoteData.Success pipeline ->
            case List.head pipeline.groups of
                Nothing ->
                    []

                Just first ->
                    [ first.name ]

        _ ->
            []


getNextUrl : List String -> Model -> String
getNextUrl newGroups model =
    pidToUrl (pipelineIdentifierFromModel model) <|
        setGroupsInLocation model.route newGroups


setGroupsInLocation : Routes.ConcourseRoute -> List String -> Routes.ConcourseRoute
setGroupsInLocation loc groups =
    let
        updatedUrl =
            if List.isEmpty groups then
                QueryString.remove "groups" loc.queries

            else
                List.foldr
                    (QueryString.add "groups")
                    QueryString.empty
                    groups
    in
    { loc | queries = updatedUrl }


pidToUrl : Maybe Concourse.PipelineIdentifier -> Routes.ConcourseRoute -> String
pidToUrl pid { queries } =
    case pid of
        Just { teamName, pipelineName } ->
            String.join ""
                [ String.join "/"
                    [ "/teams"
                    , teamName
                    , "pipelines"
                    , pipelineName
                    ]
                , QueryString.render queries
                ]

        Nothing ->
            ""


pipelineIdentifierFromModel : Model -> Maybe Concourse.PipelineIdentifier
pipelineIdentifierFromModel model =
    case model.pipeline of
        RemoteData.Success pipeline ->
            Just { teamName = pipeline.teamName, pipelineName = pipeline.name }

        _ ->
            Nothing


cliIcon : String -> List ( String, String )
cliIcon image =
    [ ( "width", "12px" )
    , ( "height", "12px" )
    , ( "background-image", "url(/public/images/" ++ image ++ "-logo.svg)" )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "background-size", "contain" )
    , ( "display", "inline-block" )
    ]
