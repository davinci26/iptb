#!/usr/bin/env python3
import json
import matplotlib.pyplot as plt
import argparse

def parse_single_line(line, filepath):
    d = json.loads(line)
    users = int(d['Users'])
    if users == len(d['Results']):
        return d['Users'], d['Avg_time'],d['Std_Time'], d['Delay_Max'], d['Delay_Min'],d['Results']
    else:
        print("The file did not reach to all nodes")

def parse_file(filepath):
    user_no = []
    delay_avg = []
    delay_std = []
    delay_max = []
    delay_min = []
    results = []
    with open(filepath) as fp:  
        line = fp.readline()
        while line:
            res = parse_single_line(line,filepath)
            if res is not None:
                user_no.append(res[0])
                delay_avg.append(res[1])
                delay_std.append(res[2])
                delay_max.append(res[3])
                delay_min.append(res[4])
                results.append(res[5])
            line = fp.readline()
    return user_no, delay_avg, delay_std, delay_max, delay_min, results


def plot(filepath,label,colour_,file_size):
    user_no, delay_avg, delay_std, delay_max, delay_min,_ = parse_file(filepath)
    plt.plot(user_no, delay_avg,'o--', color=colour_, label=label + " Average delay",ms=3) #, yerr = delay_std, fmt='o' )
    plt.plot(user_no, delay_max,'--', color=colour_,  label=label + " Max delay",alpha=0.3) 
    plt.plot(user_no, delay_min,'--', color=colour_,label=label + " Min delay",alpha=0.3) 
    plt.fill_between(user_no,
                     delay_max,
                     delay_min,
                     color =colour_,
                     alpha=0.2 )
    plt.xlabel('Number of Nodes')
    plt.ylabel('Average delay[sec]')
    if file_size:
        plt.title('Average time required to distribute a {} size file'.format(file_size))

# USAGE: ./evaluation_scripts/results_parser.py -o ipfs-vs-BitTorrent -IPFS -BitTorrent -save
if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='Command Line Interface')
    parser.add_argument('-o', type=str, nargs='?',
                        help="Save output to the specified directory")
    parser.add_argument('-i', type=str, nargs='?',
                    help="Specifiy input directory")
    
    parser.add_argument('-size', type=str, nargs='?',
                    help="Specifiy experiment file size")

    args = parser.parse_args()
    fig, ax1 = plt.subplots()
    if args.i == None:
        print("No input file is specied")
    plot(args.i, "IPFS",'blue',args.size)
    plt.legend()
    if args.o != None:
        fig.savefig(args.o +'.png',bbox_inches='tight') 
    else:
        plt.show()