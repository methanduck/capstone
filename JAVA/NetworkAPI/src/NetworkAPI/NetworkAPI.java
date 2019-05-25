package NetworkAPI;

import com.google.gson.Gson;

import java.io.*;
import java.net.*;
import java.util.ArrayList;
import java.util.Enumeration;
import java.util.List;


public class NetworkAPI {
    //Command Configuration
    private static final int SVRPORT = 6866;
    private static final String OPERATION_OPEN = "OPEN";
    private static final String OPERATION_CLOSE = "CLOSE";
    private static final String OPERATION_INFORMATION = "INFO";
    private static final String OPERATION_MODEAUTO   = "AUTO";
    private static final String COMM_OK = "NETOK";
    private static final String COMM_FAIL = "NETERR";
    private static final String SVR_ERR = "ERR";


    //NetworkAPI Commencing sequence
    //1. Connect TCP to Window which ip was provided by initialized Node class
    //2. if Window == configured, commence validation
    //2. if Window != configured, initiate window configuration based on Node class
    //3. after validation if passed, send command string to Window
    //   if error occured throws exception with string "ERRVALIDATION" or "ARGUMENTSERR"
    //   last one is occured WindowOperation method
    //4. return the result string after all precedure completed
    public String StartNetworkAPI(Node window, String order) throws Exception {
        String COMM_validationResult;
        String CommandResult;
        String COMM_receivedData = null;
        Socket COMM_Window = new Socket(window.getIPAddr(),SVRPORT);
        COMM_SendMSG(window.getIPAddr(),"HELLO",COMM_Window); // 1 HELLO
        CommandResult = COMM_RecvMSG(window.getIPAddr(), COMM_Window); //2 RECV MSG FROM SVR
        switch (COMM_receivedData){
            case "CONFIG_REQUIRE" :
                COMM_SendMSG(window.getIPAddr(), window.getPassword()+";"+window.getHostName(), COMM_Window);
                String result = COMM_RecvMSG(window.getIPAddr(),COMM_Window);
                if (result.equals(SVR_ERR))
                    throw  new Exception("ERRCONFIG");
                break;

            case "IDENTIFICATION_REQUIRE" :
                //TODO : 만약 비어있는 객체일 경우 handling
                COMM_SendMSG(window.getIPAddr(), window.getPassword(), COMM_Window);
                COMM_validationResult = COMM_RecvMSG("0", COMM_Window);
                if (COMM_validationResult.equals(SVR_ERR)){
                    throw new Exception("ERRVALIDATION");
                }
                break;
        }
        try {
            CommandResult= WindowOperations(window, order,COMM_Window);
        }catch (Exception e){
            CommandResult = e.getMessage();
        }

        COMM_Window.close();
        return CommandResult;
    }

    // if not enough argumets provided, will throws "ARGUMENTSERR" exception
    // return "NETOK" , "NETERR", required DATA i.e.) case OPERATION_INFORMATION:
    public String WindowOperations(Node Window , String Order, Socket COMM_Window) throws Exception {
        String COMM_result = COMM_OK;
        String[] splitedOrder;
        splitedOrder = Order.split(":");

        switch (splitedOrder[0])
        {
            case OPERATION_OPEN :
                COMM_SendMSG(Window.getIPAddr(),OPERATION_OPEN,COMM_Window);
                break;
            case OPERATION_CLOSE :
                COMM_SendMSG(Window.getIPAddr(), OPERATION_CLOSE,COMM_Window);
                break;
            case OPERATION_INFORMATION:
                COMM_SendMSG(Window.getIPAddr(), OPERATION_INFORMATION,COMM_Window);
                COMM_result=COMM_RecvMSG(Window.getIPAddr(), COMM_Window);
                break;
            case OPERATION_MODEAUTO :
                if (splitedOrder.length < 2) {
                    throw new Exception("ARGUMENTSERR");
                }
                COMM_SendMSG(Window.getIPAddr(), OPERATION_MODEAUTO,COMM_Window);
                break;

                default:
                    COMM_result = COMM_FAIL;
        }

        return COMM_result;
    }
    //retrieving All local smartwindow ip address with delimiter ";" ex) 192.168.0.1;192.168.0.4;
    public List<Node> FindWindow() throws IOException {
        InetAddress tmpAddr;
        int count = 0;
        List<Node> ActiveIPLIST = new ArrayList<>();

        String IP_Pattern = "((\\d{1,2}|(0|1)\\d{2}|2[0-4]\\d|25[0-5])\\.){3}(\\d{1,2}|(0|1)\\d{2}|2[0-4]\\d|25[0-5])";

        Enumeration netCard = NetworkInterface.getNetworkInterfaces();
        while (netCard.hasMoreElements()) {
            NetworkInterface nextcard = (NetworkInterface) netCard.nextElement();
            Enumeration LocalAddr = nextcard.getInetAddresses();
            while (LocalAddr.hasMoreElements()) {
                tmpAddr = (InetAddress) LocalAddr.nextElement();
                if (!(tmpAddr.getHostAddress().equals("127.0.0.1")) && (tmpAddr.getHostAddress().matches(IP_Pattern))) {
                    SubnetUtils calcAddr = new SubnetUtils(tmpAddr.getHostAddress()+"/"+Integer.toString(nextcard.getInterfaceAddresses().get(count).getNetworkPrefixLength()));
                    String[] allAddr = calcAddr.getInfo().getAllAddresses();
                    for (String targetIP : allAddr) {
                        new Thread(new Runnable() {
                            @Override
                            public void run() {
                                try {
                                    Socket Window = new Socket(Thread.currentThread().getName(),SVRPORT);
                                    Node SvrACK ;
                                    SvrACK =(Node) COMM_recvJSON(Thread.currentThread().getName(),Window);
                                    SvrACK.setIPAddr(Thread.currentThread().getName());
                                    ActiveIPLIST.add(SvrACK);
                                } catch (Exception e) {
                                }
                            }
                        },targetIP).start();
                    }
                }
                count++;
            }
        }
        return ActiveIPLIST;
    }
    //COMMUNICATION method Parameter ( string IPaddress, string Message, Socket socketObject)
    public void COMM_SendMSG(String IPAddr, String Message,Socket Window) throws IOException {
        //one time connection
        if (Window == null) {
            Socket Client = new Socket(IPAddr,6866);
            OutputStream NetOut = Client.getOutputStream();
            NetOut.write(Message.getBytes());
            Client.close();
        }else {
            OutputStream NetOut = Window.getOutputStream();
            NetOut.write(Message.getBytes());
        }

    }
    //COMMUNICATION method
    public String COMM_RecvMSG(String IPAddr,Socket Window) throws IOException {
        String result ;
        if (Window == null){
            Socket Client = new Socket(IPAddr,6866);
            InputStream NetIn = Client.getInputStream();
            BufferedReader Buff_In = new BufferedReader(new InputStreamReader(NetIn));
            Client.close();
            while ( (result=Buff_In.readLine()) != null){}
        } else{
            InputStream NetIn = Window.getInputStream();
            BufferedReader Buff_In = new BufferedReader(new InputStreamReader(NetIn));
            while ( (result=Buff_In.readLine()) != null){}
        }
        return result;
    }
    //COMMUNICATION_JSON method
    public void COMM_sendJSON(Node data,Socket Window) throws IOException {
        Gson genJSON = new Gson();
        String strJSON = genJSON.toJson(data);
        COMM_SendMSG(data.getIPAddr(),strJSON,Window);
    }
    //COMMUNICATION_JSON method
    public Object COMM_recvJSON(String IPAddr,Socket Window) throws IOException {
        Gson degenJSON = new Gson();
        String recvJSON = COMM_RecvMSG(IPAddr,Window);
        Node deserializedNodeData = degenJSON.fromJson(recvJSON,Node.class);
        return deserializedNodeData;
    }

    //**deprecated
    public String COMM_InteractiveMSG(String IPAddr, String Message) throws IOException {
        Socket Client = new Socket(IPAddr,6866);
        OutputStream NetOut = Client.getOutputStream();
        InputStream NetIn = Client.getInputStream();
        BufferedReader Buff_In = new BufferedReader(new InputStreamReader(NetIn));
        NetOut.write(Message.getBytes());
        Client.close();

        String Result = null;
        while ((Result = Buff_In.readLine())!=null) {}

        return Result;
    }

    //**deprecated
    public boolean COMM_PortCheck(String IPAddr, int Port) {
    try {
        (new Socket(IPAddr,Port)).close();
    } catch (Exception e)
    {
        return false;
    }
    return true;
    }
}
