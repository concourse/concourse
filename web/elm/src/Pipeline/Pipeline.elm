module Pipeline.Pipeline exposing
    ( Flags
    , Model
    , changeToPipelineAndGroups
    , getUpdateMessage
    , handleCallback
    , handleDelivery
    , init
    , subscriptions
    , update
    , view
    )

import Char
import Colors
import Concourse
import Concourse.Cli as Cli
import Dict
import EffectTransformer exposing (ET)
import Html exposing (Html)
import Html.Attributes
    exposing
        ( class
        , height
        , href
        , id
        , src
        , width
        )
import Html.Attributes.Aria exposing (ariaLabel)
import Html.Events exposing (onClick, onMouseEnter, onMouseLeave)
import Http
import Json.Decode
import Json.Encode
import Login
import Message.Callback exposing (Callback(..))
import Message.Effects exposing (Effect(..))
import Message.Message exposing (Hoverable(..), Message(..))
import Message.Subscription exposing (Delivery(..), Interval(..), Subscription(..))
import PauseToggle
import Pipeline.Styles as Styles
import RemoteData exposing (..)
import Routes
import StrictEvents exposing (onLeftClickOrShiftLeftClick)
import Svg exposing (..)
import Svg.Attributes as SvgAttributes
import Time exposing (Time)
import TopBar.Model exposing (PipelineState)
import TopBar.Styles
import TopBar.TopBar as TopBar
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState)


type alias Model =
    TopBar.Model.Model
        { pipelineLocator : Concourse.PipelineIdentifier
        , pipeline : WebData Concourse.Pipeline
        , fetchedJobs : Maybe Json.Encode.Value
        , fetchedResources : Maybe Json.Encode.Value
        , renderedJobs : Maybe Json.Encode.Value
        , renderedResources : Maybe Json.Encode.Value
        , concourseVersion : String
        , turbulenceImgSrc : String
        , experiencingTurbulence : Bool
        , selectedGroups : List String
        , hideLegend : Bool
        , hideLegendCounter : Time
        , isToggleHovered : Bool
        , isToggleLoading : Bool
        , isPinMenuExpanded : Bool
        }


type alias Flags =
    { pipelineLocator : Concourse.PipelineIdentifier
    , turbulenceImgSrc : String
    , selectedGroups : List String
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        ( topBar, topBarEffects ) =
            TopBar.init { route = Routes.Pipeline { id = flags.pipelineLocator, groups = flags.selectedGroups } }

        model =
            { concourseVersion = ""
            , turbulenceImgSrc = flags.turbulenceImgSrc
            , pipelineLocator = flags.pipelineLocator
            , pipeline = RemoteData.NotAsked
            , fetchedJobs = Nothing
            , fetchedResources = Nothing
            , renderedJobs = Nothing
            , renderedResources = Nothing
            , experiencingTurbulence = False
            , hideLegend = False
            , hideLegendCounter = 0
            , isToggleHovered = False
            , isToggleLoading = False
            , selectedGroups = flags.selectedGroups
            , isPinMenuExpanded = False
            , isUserMenuExpanded = topBar.isUserMenuExpanded
            , route = topBar.route
            , groups = topBar.groups
            , dropdown = topBar.dropdown
            , screenSize = topBar.screenSize
            , shiftDown = topBar.shiftDown
            }
    in
    ( model, [ FetchPipeline flags.pipelineLocator, FetchVersion, ResetPipelineFocus ] ++ topBarEffects )


changeToPipelineAndGroups :
    { pipelineLocator : Concourse.PipelineIdentifier
    , selectedGroups : List String
    }
    -> ET Model
changeToPipelineAndGroups { pipelineLocator, selectedGroups } ( model, effects ) =
    if model.pipelineLocator == pipelineLocator then
        let
            ( newModel, newEffects ) =
                renderIfNeeded ( { model | selectedGroups = selectedGroups }, [] )
        in
        ( newModel, effects ++ newEffects ++ [ ResetPipelineFocus ] )

    else
        let
            ( newModel, newEffects ) =
                init
                    { pipelineLocator = pipelineLocator
                    , selectedGroups = selectedGroups
                    , turbulenceImgSrc = model.turbulenceImgSrc
                    }
        in
        ( newModel, effects ++ newEffects )


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


handleCallback : Callback -> ET Model
handleCallback msg =
    TopBar.handleCallback msg >> handleCallbackBody msg


handleCallbackBody : Callback -> ET Model
handleCallbackBody callback ( model, effects ) =
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
            , effects
                ++ [ FetchJobs model.pipelineLocator
                   , FetchResources model.pipelineLocator
                   , SetTitle <| pipeline.name ++ " - "
                   ]
            )

        PipelineFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    if status.code == 404 then
                        ( { model | pipeline = RemoteData.Failure err }, effects )

                    else
                        ( model, effects ++ redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded ( { model | experiencingTurbulence = True }, effects )

        PipelineToggled _ (Ok ()) ->
            ( { model
                | pipeline =
                    RemoteData.map
                        (\p -> { p | paused = not p.paused })
                        model.pipeline
                , isToggleLoading = False
              }
            , effects
            )

        PipelineToggled _ (Err err) ->
            let
                newModel =
                    { model | isToggleLoading = False }
            in
            case err of
                Http.BadStatus { status } ->
                    ( newModel
                    , effects ++ redirectToLoginIfUnauthenticated status
                    )

                _ ->
                    ( newModel, effects )

        JobsFetched (Ok fetchedJobs) ->
            renderIfNeeded ( { model | fetchedJobs = Just fetchedJobs, experiencingTurbulence = False }, effects )

        JobsFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model, effects ++ redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded ( { model | fetchedJobs = Nothing, experiencingTurbulence = True }, effects )

        ResourcesFetched (Ok fetchedResources) ->
            renderIfNeeded ( { model | fetchedResources = Just fetchedResources, experiencingTurbulence = False }, effects )

        ResourcesFetched (Err err) ->
            case err of
                Http.BadStatus { status } ->
                    ( model, effects ++ redirectToLoginIfUnauthenticated status )

                _ ->
                    renderIfNeeded ( { model | fetchedResources = Nothing, experiencingTurbulence = True }, effects )

        VersionFetched (Ok version) ->
            ( { model | concourseVersion = version, experiencingTurbulence = False }, effects )

        VersionFetched (Err err) ->
            flip always (Debug.log "failed to fetch version" err) <|
                ( { model | experiencingTurbulence = True }, effects )

        _ ->
            ( model, effects )


handleDelivery : Delivery -> ET Model
handleDelivery delivery ( model, effects ) =
    case delivery of
        KeyDown keycode ->
            ( { model | hideLegend = False, hideLegendCounter = 0 }
            , if (Char.fromCode keycode |> Char.toLower) == 'f' then
                effects ++ [ ResetPipelineFocus ]

              else
                effects
            )

        Moused ->
            ( { model | hideLegend = False, hideLegendCounter = 0 }, effects )

        ClockTicked OneSecond _ ->
            if model.hideLegendCounter + timeUntilHiddenCheckInterval > timeUntilHidden then
                ( { model | hideLegend = True }, effects )

            else
                ( { model | hideLegendCounter = model.hideLegendCounter + timeUntilHiddenCheckInterval }
                , effects
                )

        ClockTicked FiveSeconds _ ->
            ( model, effects ++ [ FetchPipeline model.pipelineLocator ] )

        ClockTicked OneMinute _ ->
            ( model, effects ++ [ FetchVersion ] )

        _ ->
            ( model, effects )


update : Message -> ET Model
update msg =
    TopBar.update msg >> updateBody msg


updateBody : Message -> ET Model
updateBody msg ( model, effects ) =
    case msg of
        ToggleGroup group ->
            ( model, effects ++ [ NavigateTo <| getNextUrl (toggleGroup group model.selectedGroups model.pipeline) model ] )

        SetGroups groups ->
            ( model, effects ++ [ NavigateTo <| getNextUrl groups model ] )

        TogglePipelinePaused pipelineIdentifier isPaused ->
            ( { model | isToggleLoading = True }
            , effects
                ++ [ SendTogglePipelineRequest
                        pipelineIdentifier
                        isPaused
                   ]
            )

        Hover (Just (PipelineButton _)) ->
            ( { model | isToggleHovered = True }, effects )

        Hover (Just PinIcon) ->
            ( { model | isPinMenuExpanded = True }, effects )

        Hover Nothing ->
            ( { model | isToggleHovered = False, isPinMenuExpanded = False }
            , effects
            )

        _ ->
            ( model, effects )


getPinnedResources : Model -> List ( String, Concourse.Version )
getPinnedResources model =
    case model.fetchedResources of
        Nothing ->
            []

        Just res ->
            Json.Decode.decodeValue (Json.Decode.list Concourse.decodeResource) res
                |> Result.withDefault []
                |> List.filterMap (\r -> Maybe.map (\v -> ( r.name, v )) r.pinnedVersion)


subscriptions : Model -> List Subscription
subscriptions model =
    [ OnClockTick OneMinute
    , OnClockTick FiveSeconds
    , OnClockTick OneSecond
    , OnMouse
    , OnKeyDown
    ]


view : UserState -> Model -> Html Message
view userState model =
    let
        pipelineState =
            Just
                { pinnedResources = getPinnedResources model
                , pipeline = model.pipelineLocator
                , isPaused = isPaused model.pipeline
                , isToggleHovered = model.isToggleHovered
                , isToggleLoading = model.isToggleLoading
                }
    in
    Html.div [ Html.Attributes.style [ ( "height", "100%" ) ] ]
        [ Html.div
            [ Html.Attributes.style TopBar.Styles.pageIncludingTopBar
            , id "page-including-top-bar"
            ]
            [ Html.div
                [ id "top-bar-app"
                , Html.Attributes.style <|
                    TopBar.Styles.topBar <|
                        isPaused model.pipeline
                ]
                [ TopBar.viewConcourseLogo
                , TopBar.viewBreadcrumbs model.route
                , viewPin pipelineState model
                , viewPauseToggle userState pipelineState
                , Login.view userState model <| isPaused model.pipeline
                ]
            , Html.div
                [ Html.Attributes.style TopBar.Styles.pipelinePageBelowTopBar
                , id "page-below-top-bar"
                ]
                [ viewSubPage model ]
            ]
        ]


viewPin : Maybe PipelineState -> Model -> Html Message
viewPin pipelineState model =
    case pipelineState of
        Just { pinnedResources, pipeline } ->
            Html.div
                [ Html.Attributes.style <|
                    Styles.pinIconContainer model.isPinMenuExpanded
                , id "pin-icon"
                ]
                [ if List.length pinnedResources > 0 then
                    Html.div
                        [ Html.Attributes.style <| Styles.pinIcon
                        , onMouseEnter <| Hover <| Just PinIcon
                        , onMouseLeave <| Hover Nothing
                        ]
                        ([ Html.div
                            [ Html.Attributes.style Styles.pinBadge
                            , id "pin-badge"
                            ]
                            [ Html.div []
                                [ Html.text <|
                                    toString <|
                                        List.length pinnedResources
                                ]
                            ]
                         ]
                            ++ viewPinDropdown pinnedResources pipeline model
                        )

                  else
                    Html.div [ Html.Attributes.style <| Styles.pinIcon ] []
                ]

        Nothing ->
            Html.text ""


viewPinDropdown :
    List ( String, Concourse.Version )
    -> Concourse.PipelineIdentifier
    -> Model
    -> List (Html Message)
viewPinDropdown pinnedResources pipeline model =
    if model.isPinMenuExpanded then
        [ Html.ul
            [ Html.Attributes.style Styles.pinIconDropdown ]
            (pinnedResources
                |> List.map
                    (\( resourceName, pinnedVersion ) ->
                        Html.li
                            [ onClick <|
                                GoToRoute <|
                                    Routes.Resource
                                        { id =
                                            { teamName = pipeline.teamName
                                            , pipelineName = pipeline.pipelineName
                                            , resourceName = resourceName
                                            }
                                        , page = Nothing
                                        }
                            , Html.Attributes.style Styles.pinDropdownCursor
                            ]
                            [ Html.div
                                [ Html.Attributes.style Styles.pinText ]
                                [ Html.text resourceName ]
                            , Html.table []
                                (pinnedVersion
                                    |> Dict.toList
                                    |> List.map
                                        (\( k, v ) ->
                                            Html.tr []
                                                [ Html.td [] [ Html.text k ]
                                                , Html.td [] [ Html.text v ]
                                                ]
                                        )
                                )
                            ]
                    )
            )
        , Html.div [ Html.Attributes.style Styles.pinHoverHighlight ] []
        ]

    else
        []


viewPauseToggle : UserState -> Maybe PipelineState -> Html Message
viewPauseToggle userState pipelineState =
    case pipelineState of
        Just ({ isPaused } as ps) ->
            Html.div
                [ id "top-bar-pause-toggle"
                , Html.Attributes.style <| Styles.pauseToggle isPaused
                ]
                [ PauseToggle.view "17px" userState ps ]

        Nothing ->
            Html.text ""


isPaused : WebData Concourse.Pipeline -> Bool
isPaused p =
    RemoteData.withDefault False (RemoteData.map .paused p)


viewSubPage : Model -> Html Message
viewSubPage model =
    Html.div [ class "pipeline-view" ]
        [ viewGroupsBar model
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
            , if model.hideLegend then
                Html.text ""

              else
                Html.dl
                    [ id "legend", class "legend" ]
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
                        [ Html.ul [ class "cli-downloads" ] <|
                            List.map
                                (\cli ->
                                    Html.li []
                                        [ Html.a
                                            [ href <| Cli.downloadUrl cli
                                            , ariaLabel <| Cli.label cli
                                            , Html.Attributes.style <| cliIcon cli
                                            ]
                                            []
                                        ]
                                )
                                Cli.clis
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


viewGroupsBar : Model -> Html Message
viewGroupsBar model =
    let
        groupList =
            case model.pipeline of
                RemoteData.Success pipeline ->
                    List.map
                        (viewGroup
                            { selectedGroups = selectedGroupsOrDefault model
                            , pipelineLocator = model.pipelineLocator
                            }
                        )
                        pipeline.groups

                _ ->
                    []
    in
    if List.isEmpty groupList then
        Html.text ""

    else
        Html.nav
            [ id "groups-bar"
            , Html.Attributes.style Styles.groupsBar
            ]
            [ Html.ul
                [ Html.Attributes.style Styles.groupsList ]
                groupList
            ]


viewGroup :
    { a
        | selectedGroups : List String
        , pipelineLocator : Concourse.PipelineIdentifier
    }
    -> Concourse.PipelineGroup
    -> Html Message
viewGroup { selectedGroups, pipelineLocator } grp =
    let
        url =
            Routes.toString <|
                Routes.Pipeline { id = pipelineLocator, groups = [] }
    in
    Html.li
        []
        [ Html.a
            [ Html.Attributes.href <| url ++ "?groups=" ++ grp.name
            , Html.Attributes.style <| Styles.groupItem <| List.member grp.name selectedGroups
            , onLeftClickOrShiftLeftClick
                (SetGroups [ grp.name ])
                (ToggleGroup grp)
            ]
            [ Html.text grp.name ]
        ]


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


renderIfNeeded : ET Model
renderIfNeeded ( model, effects ) =
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
                        , effects ++ [ RenderPipeline filteredFetchedJobs fetchedResources ]
                        )

                    else
                        ( model, effects )

                _ ->
                    ( { model
                        | renderedJobs = Just filteredFetchedJobs
                        , renderedResources = Just fetchedResources
                      }
                    , effects ++ [ RenderPipeline filteredFetchedJobs fetchedResources ]
                    )

        _ ->
            ( model, effects )


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


selectedGroupsOrDefault : Model -> List String
selectedGroupsOrDefault model =
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
    Routes.toString <|
        Routes.Pipeline { id = model.pipelineLocator, groups = newGroups }


cliIcon : Cli.Cli -> List ( String, String )
cliIcon cli =
    [ ( "width", "12px" )
    , ( "height", "12px" )
    , ( "background-image", Cli.iconUrl cli )
    , ( "background-repeat", "no-repeat" )
    , ( "background-position", "50% 50%" )
    , ( "background-size", "contain" )
    , ( "display", "inline-block" )
    ]
